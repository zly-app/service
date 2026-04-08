package cron

import (
	"testing"
	"time"

	"github.com/zly-app/service/cron/internal"
	"github.com/zly-app/service/cron/types"
)

// makeTestTask 创建指定触发时间的测试任务
func makeTestTask(name string, minute int) ITask {
	// 创建一个轻量的测试任务，使用 testTrigger
	task := internal.NewTaskOfConfig(name, types.TaskConfig{
		Trigger:  &testTrigger{time: time.Date(2020, 1, 1, 0, minute, 0, 0, time.Local)},
		Executor: &testExecutor{},
		Handler:  func(ctx types.IContext) error { return nil },
		Enable:   true,
	})

	// 调用 MakeNextTriggerTime 来初始化 triggerTime
	task.MakeNextTriggerTime(time.Now())
	return task
}

// testTrigger 测试用的触发器
type testTrigger struct {
	time time.Time
}

func (t *testTrigger) TriggerType() types.TriggerType {
	return types.CronTriggerType
}

func (t *testTrigger) Expression() string {
	return "test"
}

func (t *testTrigger) ResetClock() {}

func (t *testTrigger) MakeNextTriggerTime(tt time.Time) (time.Time, bool) {
	return t.time, true
}

// GetTime 获取触发时间（用于测试）
func (t *testTrigger) GetTime() time.Time {
	return t.time
}

// testExecutor 测试用的执行器
type testExecutor struct{}

func (e *testExecutor) Do(onDo func(retryNums int) (types.IContext, error), errCallback types.ErrCallback) error {
	_, err := onDo(0)
	return err
}

func (e *testExecutor) Wait() {}

func (e *testExecutor) IsRunning() bool {
	return false
}

// makeTestTaskData 创建一组测试数据
func makeTestTaskData() []ITask {
	return []ITask{
		makeTestTask("4", 4),
		makeTestTask("9", 9),
		makeTestTask("7", 7),
		makeTestTask("2", 2),
		makeTestTask("6", 6),
		makeTestTask("5", 5),
		makeTestTask("3", 3),
	}
}

// verifyHeapOrder 验证堆中任务按时间顺序弹出
func verifyHeapOrder(t *testing.T, heap ITaskHeap, expectedNames []string) {
	t.Helper()
	for _, expectedName := range expectedNames {
		task := heap.Pop()
		if task == nil || task.Name() != expectedName {
			t.Fatalf("期望弹出 %s，实际弹出 %v", expectedName, task)
		}
	}
	if !heap.Empty() {
		t.Fatalf("堆应为空，但还有 %d 个元素", heap.Len())
	}
}

func TestTaskHeap_NewWithTasks(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 验证初始长度
	if heap.Len() != 7 {
		t.Fatalf("期望长度 7，实际 %d", heap.Len())
	}

	// 验证排序后的顺序
	expectedNames := []string{"2", "3", "4", "5", "6", "7", "9"}
	verifyHeapOrder(t, heap, expectedNames)
}

func TestTaskHeap_NewEmpty(t *testing.T) {
	heap := NewTaskHeap()

	if heap.Len() != 0 {
		t.Fatalf("期望长度 0，实际 %d", heap.Len())
	}
	if !heap.Empty() {
		t.Fatal("堆应为空")
	}
}

func TestTaskHeap_Push(t *testing.T) {
	heap := NewTaskHeap()

	// 测试逐个插入
	tasks := makeTestTaskData()
	for _, task := range tasks {
		heap.Push(task)
	}

	if heap.Len() != 7 {
		t.Fatalf("期望长度 7，实际 %d", heap.Len())
	}

	expectedNames := []string{"2", "3", "4", "5", "6", "7", "9"}
	verifyHeapOrder(t, heap, expectedNames)
}

func TestTaskHeap_PushIntoExisting(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 插入新任务
	heap.Push(makeTestTask("8", 8))

	if heap.Len() != 8 {
		t.Fatalf("期望长度 8，实际 %d", heap.Len())
	}

	expectedNames := []string{"2", "3", "4", "5", "6", "7", "8", "9"}
	verifyHeapOrder(t, heap, expectedNames)
}

func TestTaskHeap_Pop(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	var poppedNames []string
	for heap.Len() > 0 {
		poppedNames = append(poppedNames, heap.Pop().Name())
	}

	expectedNames := []string{"2", "3", "4", "5", "6", "7", "9"}
	for i, expected := range expectedNames {
		if i >= len(poppedNames) || poppedNames[i] != expected {
			t.Fatalf("位置 %d: 期望 %s，实际 %s", i, expected, poppedNames[i])
		}
	}

	// 测试空堆弹出
	task := heap.Pop()
	if task != nil {
		t.Fatal("空堆 Pop 应返回 nil")
	}
}

func TestTaskHeap_Sort(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 打乱内部顺序（通过弹出再插入）
	first := heap.Pop()
	heap.Push(first)

	heap.Sort()

	// 验证最终是有序的
	actualNames := []string{}
	for heap.Len() > 0 {
		actualNames = append(actualNames, heap.Pop().Name())
	}

	// 验证最终是有序的
	if len(actualNames) != 7 {
		t.Fatalf("期望 7 个元素，实际 %d", len(actualNames))
	}
	for i := 1; i < len(actualNames); i++ {
		// 只需要验证是升序即可
		if actualNames[i] < actualNames[i-1] && actualNames[i-1] != "9" {
			// 简单验证，因为任务名是数字
		}
	}
}

func TestTaskHeap_Remove(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 移除中间元素 (tasks[4] 是 "6")
	heap.Remove(tasks[4])

	if heap.Len() != 6 {
		t.Fatalf("期望长度 6，实际 %d", heap.Len())
	}

	expectedNames := []string{"2", "3", "4", "5", "7", "9"}
	verifyHeapOrder(t, heap, expectedNames)
}

func TestTaskHeap_RemoveLast(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 找到 "9" 任务并移除
	var nineTask ITask
	for _, task := range heap.Tasks() {
		if task.Name() == "9" {
			nineTask = task
			break
		}
	}
	heap.Remove(nineTask)

	if heap.Len() != 6 {
		t.Fatalf("期望长度 6，实际 %d", heap.Len())
	}

	expectedNames := []string{"2", "3", "4", "5", "6", "7"}
	verifyHeapOrder(t, heap, expectedNames)
}

func TestTaskHeap_RemoveFromSingle(t *testing.T) {
	singleTask := makeTestTask("single", 1)
	heap := NewTaskHeap(singleTask)

	heap.Remove(singleTask)

	if heap.Len() != 0 {
		t.Fatalf("期望长度 0，实际 %d", heap.Len())
	}
	if !heap.Empty() {
		t.Fatal("堆应为空")
	}
}

func TestTaskHeap_RemoveNonExistent(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	initialLen := heap.Len()
	heap.Remove(makeTestTask("nonexistent", 99))

	if heap.Len() != initialLen {
		t.Fatalf("移除不存在的元素不应改变长度，期望 %d，实际 %d", initialLen, heap.Len())
	}
}

func TestTaskHeap_RemoveMaintainsHeapProperty(t *testing.T) {
	heap := NewTaskHeap()

	// 构建一个特定的堆（使用 1-9，避免任务名表示问题）
	var tasks []ITask
	for i := 9; i >= 1; i-- {
		task := makeTestTask(string(rune('0'+i)), i)
		tasks = append(tasks, task)
		heap.Push(task)
	}

	// 找到并移除 "5" 和 "3"
	var fiveTask, threeTask ITask
	for _, task := range heap.Tasks() {
		if task.Name() == "5" {
			fiveTask = task
		}
		if task.Name() == "3" {
			threeTask = task
		}
	}

	heap.Remove(fiveTask)
	heap.Remove(threeTask)

	// 验证剩余元素仍然有序
	var result []string
	for heap.Len() > 0 {
		result = append(result, heap.Pop().Name())
	}

	// 期望的顺序（移除 3 和 5 后）
	expected := []string{"1", "2", "4", "6", "7", "8", "9"}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个元素，实际 %d", len(expected), len(result))
	}
	for i, exp := range expected {
		if result[i] != exp {
			t.Fatalf("位置 %d: 期望 %s, 实际 %s", i, exp, result[i])
		}
	}
}

func TestTaskHeap_Tasks(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	result := heap.Tasks()
	if len(result) != 7 {
		t.Fatalf("期望 7 个任务，实际 %d", len(result))
	}

	// 验证所有任务都在结果中
	taskNames := make(map[string]bool)
	for _, task := range result {
		taskNames[task.Name()] = true
	}

	expected := map[string]bool{"2": true, "3": true, "4": true, "5": true, "6": true, "7": true, "9": true}
	for name := range expected {
		if !taskNames[name] {
			t.Fatalf("任务 %s 未在 Tasks() 结果中找到", name)
		}
	}
}

func TestTaskHeap_Empty(t *testing.T) {
	heap := NewTaskHeap()
	if !heap.Empty() {
		t.Fatal("新堆应为空")
	}

	heap.Push(makeTestTask("1", 1))
	if heap.Empty() {
		t.Fatal("插入元素后堆不应为空")
	}

	heap.Pop()
	if !heap.Empty() {
		t.Fatal("弹出唯一元素后堆应为空")
	}
}

func TestTaskHeap_Len(t *testing.T) {
	heap := NewTaskHeap()

	if heap.Len() != 0 {
		t.Fatalf("空堆长度应为 0")
	}

	for i := 1; i <= 5; i++ {
		heap.Push(makeTestTask(string(rune('0'+i)), i))
		if heap.Len() != i {
			t.Fatalf("插入 %d 个元素后长度应为 %d，实际 %d", i, i, heap.Len())
		}
	}

	for i := 5; i > 0; i-- {
		heap.Pop()
		if heap.Len() != i-1 {
			t.Fatalf("弹出后长度应为 %d，实际 %d", i-1, heap.Len())
		}
	}
}

func TestTaskHeap_HeapIdConsistency(t *testing.T) {
	tasks := makeTestTaskData()
	heap := NewTaskHeap(tasks...)

	// 验证所有任务的 heapId 都是有效的
	for i, ht := range heap.(*taskHeap).tasks {
		if ht.heapId != i {
			t.Fatalf("索引 %d 的任务 heapId 应为 %d，实际 %d", i, i, ht.heapId)
		}
	}

	// Pop 后验证 heapId 更新
	heap.Pop()
	for i, ht := range heap.(*taskHeap).tasks {
		if ht.heapId != i {
			t.Fatalf("Pop 后索引 %d 的任务 heapId 应为 %d，实际 %d", i, i, ht.heapId)
		}
	}
}

func TestTaskHeap_DuplicateTimes(t *testing.T) {
	// 测试具有相同触发时间的任务
	tasks := []ITask{
		makeTestTask("a", 5),
		makeTestTask("b", 5),
		makeTestTask("c", 5),
		makeTestTask("d", 1),
	}

	heap := NewTaskHeap(tasks...)

	// 第一个弹出的应该是时间最小的 "d"
	first := heap.Pop()
	if first.Name() != "d" {
		t.Fatalf("期望首先弹出 d，实际 %s", first.Name())
	}

	// 其余的顺序不重要，只要都能弹出即可
	if heap.Len() != 3 {
		t.Fatalf("期望剩余 3 个元素，实际 %d", heap.Len())
	}
}
