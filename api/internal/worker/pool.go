package worker
import (
	"context"
	"log/slog"
)

//Handler will process a single job identified by its ID. 
type Handler func(ctx context.Context, jobID string) error 

//A simple-default worker pool that consumes job IDs from a buffered channel
//It exists to remove long-running ingestion work from the REQUEST and fire as a go-routine
//so that asynchronous behaviour can be implemented in our POST /repos end-point
type Pool struct {
	jobs 		chan string 
	handler     Handler
	logger 		*slog.Logger 
	size        int 
}

func NewPool(bufferSize, size int, handler Handler, logger *slog.Logger) *Pool {
	return &Pool{
		jobs:		make(chan string, bufferSize),
		handler:	handler,
		logger:		logger, 
		size:		size, 
	}
}

//This function launches the worker goroutine. It returns an acknowledgement immediately - workers run untill ctx is cancelled or Stop() closes the job channel
func(p *Pool) Start(ctx context.Context) {

	for i:= 0; i < p.size; i++ {

		workerID := i

		go p.runWorker(ctx, workerID)
	}

	p.logger.Info("worker_pool_started", "workers", p.size)
}

func(p *Pool) runWorker(ctx context.Context, workerID int) {

	for {
		select {
		//Context is cancelled
		case <-ctx.Done():
			p.logger.Info("worker_stopped", "worker_id", workerID)
			return 
		case jobID, ok := <-p.jobs:
			//Job's channel is close
			if !ok {
				p.logger.Info("worker_stopped", "worker_id", workerID, "reason", "channel_closes")
				return 
			}

			p.logger.Info("worker_job_started", "worker_id", workerID, "job_id", jobID)

			if err := p.handler(ctx, jobID); err != nil{
				p.logger.Error("worker_job_failed", "worker_id", workerID, "job_id", jobID, "error", err)
				continue
			}

			p.logger.Info("worker_job_completed", "worker_id", workerID, "job_id", jobID)
		}
	}
}

//Enqueue function takes in a job for background processing.
//It returns False if the queue if full so the caller gets 503 instead of blocking the requests
func(p *Pool)Enqueue(jobID string) bool {

	select{
	case p.jobs <- jobID:
		return true 
	default:
		return false 
	}
}


func(p *Pool) Stop() {
	close(p.jobs)
}