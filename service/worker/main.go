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
	popJobAPI    = "/internal/pop_job"
	updateJobAPI = "/internal/jobs/%d"
)

func main() {
	flag.Parse()
	tick := time.Tick(3 * time.Second)
	for range tick {
		newJob, err := getNewJobs()
		if err != nil {
			fmt.Printf("get new jobs error: %s\n", err)
			continue
		}
		if newJob == nil {
			continue
		}

		go runJob(newJob)
	}
}

func runJob(job *model.Job) {
	fmt.Printf("received new job: %d\n", job.ID)
	job.Status = model.JobStatus_Running

	if err := updateJob(job); err != nil {
		fmt.Printf("update job error: %s\n", err)
		return
	}

	logPrintf := func(format string, args ...interface{}) {
		logPrefix := fmt.Sprintf("Job [%d]: ", job.ID)
		log := logPrefix + fmt.Sprintf(format, args...)
		job.Logs = append(job.Logs, log)
		fmt.Println(log)
	}

	logPrintf("starting")
	logPrintf("running")
	cmd := exec.Command(job.Command[0], job.Command[1:]...)

	outputs, err := cmd.CombinedOutput()

	for _, line := range bytes.Split(outputs, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		logPrintf(string(line))
	}

	if err != nil {
		logPrintf("failed: %s", err)
		job.Status = model.JobStatus_Failed
	} else {
		logPrintf("finished")
		job.Status = model.JobStatus_Done
	}

	if err := updateJob(job); err != nil {
		fmt.Printf("update job error: %s\n", err)
	}
}

func updateJob(job *model.Job) error {
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

func getNewJobs() (*model.Job, error) {
	response, err := http.Get(strings.Join([]string{"http://", *server, ":", strconv.Itoa(*port), popJobAPI}, ""))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNoContent {
		return nil, nil
	}

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

	var newJob model.Job
	if err := json.Unmarshal(data, &newJob); err != nil {
		return nil, fmt.Errorf("unmarshal (%s) to job error: %s", string(data), err)
	}
	return &newJob, nil
}
