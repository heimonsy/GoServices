package main

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	"github.com/heimonsy/GoServices/lib/model"
)

func main() {
	storage := &Storage{incr: 10000, jobs: make(map[int]*model.Job)}
	e := echo.New()
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "method=${method}, uri=${uri}, status=${status}\n",
	}))

	// createa job
	e.POST("/api/jobs", func(c echo.Context) error {
		var req model.Job
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, newJsonErrorf("parse request error: %s", err))
		}
		if len(req.Command) == 0 {
			return c.JSON(http.StatusBadRequest, newJsonErrorf("command is empty"))
		}

		return c.JSON(http.StatusOK, storage.CreateJob(req))
	})

	// list job
	e.GET("/api/jobs", func(c echo.Context) error {
		jobs := storage.ListJobs()
		return c.JSON(http.StatusOK, jobs)
	})

	// get a job
	e.GET("/api/jobs/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, newJsonErrorf("parse id %s error: %s", c.Param("id"), err))
		}

		job := storage.GetJob(id)
		return c.JSON(http.StatusOK, job)
	})

	// update a job
	e.PUT("/internal/jobs/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, newJsonErrorf("parse id %s error: %s", c.Param("id"), err))
		}
		var req model.Job
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, newJsonErrorf("parse request error: %s", err))
		}

		if err := storage.UpdateJob(id, req.Status, req.Logs); err != nil {
			return c.JSON(http.StatusInternalServerError, newJsonErrorf("update job error: %s", err))
		}
		return c.NoContent(http.StatusOK)
	})

	// pop a pending job
	e.GET("/internal/pop_job", func(c echo.Context) error {

		job := storage.PopPendingJob()
		if job == nil {
			return c.NoContent(http.StatusNoContent)
		}
		return c.JSON(http.StatusOK, job)
	})

	e.Logger.Fatal(e.Start(":1323"))
}

type Storage struct {
	mu   sync.Mutex
	incr int
	jobs map[int]*model.Job
}

func (s *Storage) CreateJob(job model.Job) model.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incr++
	job.ID = s.incr
	job.Status = model.JobStatus_Created
	job.CreatedAt = time.Now()
	job.UpdatedAt = job.CreatedAt
	s.jobs[job.ID] = &job
	return job
}

func (s *Storage) GetJob(id int) model.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.jobs[id]
}

func (s *Storage) ListJobs() []model.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobs := make([]model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, *job)
	}
	return jobs
}

func (s *Storage) UpdateJob(id int, status string, logs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return fmt.Errorf("job %d not found", id)
	}
	job.Status = status
	job.Logs = logs
	job.UpdatedAt = time.Now()
	return nil
}

// PopJob pops a pending job from the list
func (s *Storage) PopPendingJob() *model.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.Status != model.JobStatus_Created {
			continue
		}
		job.Status = model.JobStatus_Pending
		job.UpdatedAt = time.Now()
		var retJob model.Job
		retJob = *job
		return &retJob
	}
	return nil
}

func newJsonErrorf(msg string, args ...interface{}) interface{} {
	return model.Error{Message: fmt.Sprintf(msg, args...)}
}
