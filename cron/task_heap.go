// Package cron 实现了一个基于最小堆的任务调度器
// task_heap.go 提供了任务堆的实现，用于按触发时间管理定时任务的优先队列
//
// 实现原理:
// - 使用数组存储完全二叉树，索引 i 的父节点为 (i-1)/2，左子节点为 2*i+1，右子节点为 2*i+2
// - 维护最小堆性质：父节点的触发时间总是小于等于子节点
// - 每个 heapTask 维护 heapId 索引，支持 O(log n) 的 Remove 操作
//
// 时间复杂度:
// - Push: O(log n)
// - Pop: O(log n)
// - Remove: O(n) 查找 + O(log n) 调整 = O(n)
// - Sort: O(n log n)

package cron

import "sort"

// ITaskHeap 定义了任务堆的接口
// 用于管理按触发时间排序的任务集合，支持快速访问最早需要执行的任务
type ITaskHeap interface {
	// 堆排序
	Sort()
	// 弹出第一个
	Pop() ITask
	// 压入
	Push(task ITask)
	// 移除
	Remove(task ITask)
	// 获取任务列表
	Tasks() []ITask
	// 是否为空
	Empty() bool
	// 长度
	Len() int
}

// taskHeap 是任务堆的实现
type taskHeap struct {
	tasks []*heapTask
}

func NewTaskHeap(tasks ...ITask) ITaskHeap {
	h := &taskHeap{}
	for _, task := range tasks {
		h.tasks = append(h.tasks, &heapTask{task: task, heapId: -1})
	}
	h.Sort()
	return h
}

func (h *taskHeap) Sort() {
	sort.Slice(h.tasks, func(i, j int) bool {
		return h.less(i, j)
	})
	// 更新堆索引
	for i, t := range h.tasks {
		t.heapId = i
	}
}

func (h *taskHeap) Pop() ITask {
	if len(h.tasks) == 0 {
		return nil
	}

	// 保存要弹出的任务
	task := h.tasks[0]

	// 替换第一个元素为最后一个
	last := h.tasks[len(h.tasks)-1]
	last.heapId = 0
	h.tasks[0] = last
	h.tasks = h.tasks[:len(h.tasks)-1]

	// 下沉调整堆
	if len(h.tasks) > 0 {
		h.down(0)
	}

	task.heapId = -1 // 标记为不在堆中
	return task.task
}

func (h *taskHeap) Push(task ITask) {
	ht := &heapTask{task: task, heapId: -1}
	h.tasks = append(h.tasks, ht)
	h.up(len(h.tasks) - 1)
}

func (h *taskHeap) Remove(task ITask) {
	// 找到任务在堆中的索引
	index := -1
	for i, t := range h.tasks {
		if t.task == task {
			index = i
			break
		}
	}

	if index == -1 || index >= len(h.tasks) {
		return
	}

	// 如果要移除的是最后一个元素，直接删除
	if index == len(h.tasks)-1 {
		h.tasks = h.tasks[:len(h.tasks)-1]
		return
	}

	// 用最后一个元素替换被删除的元素
	last := h.tasks[len(h.tasks)-1]
	h.tasks[index] = last
	last.heapId = index
	h.tasks = h.tasks[:len(h.tasks)-1]

	// 重新调整堆：先尝试下沉，如果未移动则尝试上浮
	if !h.downFix(index) {
		h.up(index)
	}
}

// downFix 执行下沉操作，返回是否发生了交换
func (h *taskHeap) downFix(i0 int) bool {
	swapped := false
	i := i0
	n := len(h.tasks)
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && h.less(j2, j1) {
			j = j2
		}
		if !h.less(j, i) {
			break
		}
		h.swap(i, j)
		swapped = true
		i = j
	}
	return swapped
}

func (h *taskHeap) Tasks() []ITask {
	result := make([]ITask, len(h.tasks))
	for i, t := range h.tasks {
		result[i] = t.task
	}
	return result
}

func (h *taskHeap) Empty() bool {
	return len(h.tasks) == 0
}

func (h *taskHeap) Len() int {
	return len(h.tasks)
}

func (h *taskHeap) less(i, j int) bool {
	return h.tasks[i].TriggerTime().Before(h.tasks[j].TriggerTime())
}

func (h *taskHeap) swap(i, j int) {
	h.tasks[i], h.tasks[j] = h.tasks[j], h.tasks[i]
	h.tasks[i].heapId = i
	h.tasks[j].heapId = j
}

func (h *taskHeap) up(j int) {
	for {
		i := (j - 1) / 2 // parent
		if i == j || !h.less(j, i) {
			break
		}
		h.swap(i, j)
		j = i
	}
}

func (h *taskHeap) down(i0 int) {
	i := i0
	n := len(h.tasks)
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && h.less(j2, j1) {
			j = j2 // right child
		}
		if !h.less(j, i) {
			break
		}
		h.swap(i, j)
		i = j
	}
}
