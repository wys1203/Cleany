package manager

import (
	"context"
	"log"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	cleanyv1alpha1 "github.com/wys1203/Cleany/api/cleany/v1alpha1"
	"github.com/wys1203/Cleany/internal/executor"
)

const (
	StatusInQueue = "in-queue"
	StatusRunning = "running"
	StatusDone    = "done"
)

type Task struct {
	Name   string
	Status string

	Cleaner *cleanyv1alpha1.Cleaner
}

type CleanerManager struct {
	manager.Manager

	// workerCount is the number of workers to run the cleaner
	workerCount int

	// taskQueue is the queue of tasks to be cleaned
	taskQueue chan *Task

	// taskStatus is the status of the task
	taskStatus map[string]*Task

	taskStatusMu sync.Mutex
}

func NewCleanerManager(m manager.Manager, workerCount int) *CleanerManager {
	return &CleanerManager{
		Manager:     m,
		workerCount: workerCount,
		taskQueue:   make(chan *Task, 2000),
		taskStatus:  make(map[string]*Task),
	}
}

func (c *CleanerManager) Start(ctx context.Context) error {
	for i := 0; i < c.workerCount; i++ {
		go c.worker(ctx)
	}

	return c.Manager.Start(ctx)
}

func (c *CleanerManager) AddTask(task *Task) bool {
	c.taskStatusMu.Lock()
	defer c.taskStatusMu.Unlock()

	// check if the task is already in the queue or running or done
	if _, ok := c.taskStatus[task.Name]; ok {
		return false
	}

	// add the task to the queue
	task.Status = StatusInQueue
	c.taskStatus[task.Name] = task

	c.taskQueue <- task
	return true
}

func (c *CleanerManager) GetTaskStatus(name string) *Task {
	c.taskStatusMu.Lock()
	defer c.taskStatusMu.Unlock()

	if task, ok := c.taskStatus[name]; ok {
		return task
	}
	return nil
}

func (c *CleanerManager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Main context cancelled, but don't return immediately.
			// Instead, proceed to check if there's a running task that needs to complete.
		case task := <-c.taskQueue:
			c.taskStatusMu.Lock()
			task.Status = StatusRunning
			c.taskStatus[task.Name] = task
			c.taskStatusMu.Unlock()

			// Do the cleaning
			taskCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)

			go func() {
				exe, err := executor.NewExecutor(taskCtx, task.Name, c.Manager.GetConfig(), c.Manager.GetClient(), c.Manager.GetScheme())
				if err != nil {
					log.Printf("error creating executor for %s: %v", task.Name, err)
					cancel()
					return
				}
				if err := exe.Run(taskCtx); err != nil {
					log.Printf("error cleaning %s: %v", task.Name, err)
				}
				cancel() // Ensure resources are released once Execute is done.
			}()

			// Wait for either the task to complete or the main context to be cancelled.
			select {
			case <-taskCtx.Done():
				// Task completed or task context cancelled (due to timeout or main context cancellation).
			case <-ctx.Done():
				// Main context cancelled. Wait for the task to complete.
				<-taskCtx.Done()
			}

			c.taskStatusMu.Lock()
			task.Status = StatusDone
			c.taskStatus[task.Name] = task
			c.taskStatusMu.Unlock()

			if ctx.Err() != nil {
				// If the main context is cancelled, exit the loop and end the worker.
				return
			}
		}
	}
}
