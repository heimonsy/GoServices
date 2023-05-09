package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/heimonsy/GoServices/lib/model"
)

var (
	server = flag.String("server", "localhost", "server to connect to")
	port   = flag.Int("port", 1323, "port to connect to")
)

const (
	listJobsAPI  = "/api/jobs"
	updateJobAPI = "/internal/jobs/%d"
)

func main() {
	flag.Parse()
	tick := time.Tick(3 * time.Second)
	for range tick {
		newJobs, err := getNewJobs()
		if err != nil {
			fmt.Printf("get new jobs error: %s\n", err)
			continue
		}

		for _, job := range newJobs {
			go runJob(job)
		}
	}
}

func runJob(job model.Job) {
	fmt.Printf("received new job: %d\n", job.ID)
	job.Status = model.JobStatus_Running

	if err := updateJob(job); err != nil {
		fmt.Printf("update job error: %s\n", err)
		return
	}

	logPrintf := func(format string, args ...interface{}) {
		log := fmt.Sprintf(format, args...)
		job.Logs = append(job.Logs, log)
		fmt.Println(log)
	}

	logPrintf("job [%d] starting", job.ID)
	logPrintf("job [%d] running: %s", job.ID, strings.Join(job.Command, " "))
	cmd := exec.Command(job.Command[0], job.Command[1:]...)

	outputs, err := cmd.CombinedOutput()

	for _, line := range bytes.Split(outputs, []byte("\n")) {
		job.Logs = append(job.Logs, string(line))
	}

	if err != nil {
		logPrintf("job [%s] failed: %s", job.ID, err)
		job.Status = model.JobStatus_Failed
	} else {
		logPrintf("job [%d] finished", job.ID)
		job.Status = model.JobStatus_Done
	}

	if err := updateJob(job); err != nil {
		fmt.Printf("update job error: %s\n", err)
	}
}

func updateJob(job model.Job) error {
	reqBody, err := json.Marshal(model.Job{
		ID:     job.ID,
		Status: job.Status,
		Logs:   job.Logs,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, strings.Join([]string{"http://", *server, ":", strconv.Itoa(*port), fmt.Sprintf(updateJobAPI, job.ID)}, ""), bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}

		var serverError model.Error
		if err := json.Unmarshal(data, &serverError); err != nil {
			return fmt.Errorf("unmarshal (%s) error: %s", string(data), err)
		}

		return fmt.Errorf("get error from server: %s", serverError.Message)
	}
	return nil
}

func getNewJobs() ([]model.Job, error) {
	response, err := http.Get(strings.Join([]string{"http://", *server, ":", strconv.Itoa(*port), listJobsAPI}, ""))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		var serverError model.Error
		if err := json.Unmarshal(data, &serverError); err != nil {
			return nil, fmt.Errorf("unmarshal (%s) error: %s", string(data), err)
		}

		return nil, fmt.Errorf("get error from server: %s", serverError.Message)
	}
	var jobs []model.Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, fmt.Errorf("unmarshal (%s) to jobs error: %s", string(data), err)
	}

	var retJobs []model.Job
	for _, job := range jobs {
		if job.Status == model.JobStatus_Pending {
			retJobs = append(retJobs, job)
		}
	}
	return retJobs, nil
}
