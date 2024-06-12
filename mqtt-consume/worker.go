package mqtt_consume

type Worker struct {
	workers chan *Worker
	jobPool chan func()
}

type Workers struct {
	workers chan *Worker
}

func (w *Worker) Start() {
	for {
		select {
		case job := <-w.jobPool:
			if job == nil {
				return
			}
			job()
			w.workers <- w
		}
	}
}

func (w *Workers) Start() {
	count := cap(w.workers)
	for i := 0; i < count; i++ {
		worker := &Worker{
			workers: w.workers,
			jobPool: make(chan func()),
		}
		go worker.Start()
		w.workers <- worker
	}
}

// 停止工作, 会等待已添加的job执行完毕
func (w *Workers) Stop() {
	count := cap(w.workers)
	for i := 0; i < count; i++ {
		worker := <-w.workers
		worker.jobPool <- nil
	}
}

func (w *Workers) Go(job func()) {
	if job == nil {
		return
	}
	worker := <-w.workers
	worker.jobPool <- job
}

func NewWorkers(count int) *Workers {
	if count < 1 {
		count = 1
	}
	workers := make(chan *Worker, count)
	return &Workers{
		workers: workers,
	}
}
