package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mediacli/pkg/core"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func urlPath(value string) string { return url.PathEscape(value) }

func query(value string) string { return url.QueryEscape(value) }

func (cli *app) request(method, path string, payload any) (json.RawMessage, error) {
	endpoint, err := url.Parse(cli.server)
	if err != nil || endpoint.Host == "" || endpoint.Scheme != "http" && endpoint.Scheme != "https" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, usage("invalid --server URL")
	}
	var body []byte
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
	}
	request, err := http.NewRequestWithContext(cli.ctx, method, strings.TrimRight(cli.server, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if cli.token != "" {
		request.Header.Set("Authorization", "Bearer "+cli.token)
	}
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return fmt.Errorf("daemon redirect rejected") }}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("daemon HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("daemon returned invalid JSON")
	}
	return data, nil
}

func (cli *app) remote(method, path string, payload any) error {
	data, err := cli.request(method, path, payload)
	if err != nil {
		return err
	}
	return cli.emit(data)
}

func pipelinePayload(p core.Pipeline) (json.RawMessage, error) { return core.MarshalPipeline(p) }

func (cli *app) remotePipeline(p core.Pipeline, preview float64) error {
	data, err := pipelinePayload(p)
	if err != nil {
		return err
	}
	if preview != 0 {
		if cli.background {
			return usage("--background is not supported with preview")
		}
		var body map[string]any
		json.Unmarshal(data, &body)
		body["duration"] = strconv.FormatFloat(preview, 'f', -1, 64)
		if cli.dry {
			return cli.emit(body)
		}
		return cli.remote("POST", "/api/pipeline/preview", body)
	}
	if cli.dry {
		return cli.remote("POST", "/api/pipeline/dry-run", data)
	}
	result, err := cli.request("POST", "/api/pipeline/run", data)
	if err != nil {
		return err
	}
	return cli.submitted(result)
}

func (cli *app) remoteBatch(job core.BatchJob, skipInvalid bool) error {
	pipeline, err := pipelinePayload(job.Pipeline)
	if err != nil {
		return err
	}
	payload := map[string]any{"inputs": job.Inputs, "pipeline": pipeline, "output_dir": job.OutputDir, "output_template": job.OutputTemplate, "overwrite": job.Overwrite, "skip_invalid": skipInvalid, "dry_run": cli.dry}
	result, err := cli.request("POST", "/api/batch", payload)
	if err != nil {
		return err
	}
	if cli.dry {
		return cli.emit(result)
	}
	return cli.submitted(result)
}

func (cli *app) remoteDownloads(addresses []string, directory string, playlist bool, fields map[string]any) error {
	if cli.dry {
		return cli.emit(map[string]any{"urls": addresses, "output_dir": directory, "fields": fields})
	}
	for _, address := range addresses {
		result, err := cli.request("POST", "/api/downloads", map[string]any{"url": address, "out_dir": directory, "is_playlist": playlist, "fields": fields, "transcode_mode": "embedded"})
		if err != nil {
			return err
		}
		if err := cli.submitted(result); err != nil {
			return err
		}
	}
	return nil
}

func (cli *app) submitted(result json.RawMessage) error {
	if cli.background {
		return cli.emit(result)
	}
	var response struct {
		ID  int   `json:"task_id"`
		IDs []int `json:"task_ids"`
	}
	if err := json.Unmarshal(result, &response); err != nil {
		return err
	}
	if response.ID != 0 {
		response.IDs = append(response.IDs, response.ID)
	}
	if len(response.IDs) == 0 {
		return fmt.Errorf("daemon returned no task IDs")
	}
	for _, id := range response.IDs {
		if err := cli.waitRemote(id); err != nil {
			cleanup := *cli
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cleanup.ctx = ctx
			defer cancel()
			for _, pending := range response.IDs {
				cleanup.request("POST", fmt.Sprintf("/api/tasks/%d/cancel", pending), nil)
			}
			return err
		}
	}
	return cli.emit(result)
}

func (cli *app) waitRemote(id int) error {
	cursor := 0
	for {
		data, err := cli.request("GET", fmt.Sprintf("/api/tasks/%d", id), nil)
		if err != nil {
			return err
		}
		var task core.TaskSnapshot
		if err := json.Unmarshal(data, &task); err != nil {
			return err
		}
		logData, err := cli.request("GET", fmt.Sprintf("/api/tasks/%d/logs?after=%d", id, cursor), nil)
		if err != nil {
			return err
		}
		var logBatch struct {
			Logs  []string `json:"logs"`
			Next  int      `json:"next"`
			Total int      `json:"total"`
		}
		if err := json.Unmarshal(logData, &logBatch); err != nil {
			return err
		}
		for _, line := range logBatch.Logs {
			fmt.Fprintln(cli.errOut, line)
		}
		cursor = logBatch.Next
		if cursor < logBatch.Total {
			continue
		}
		switch task.Status {
		case core.StatusDone:
			return nil
		case core.StatusFailed, core.StatusCancelled, core.StatusInterrupted:
			return fmt.Errorf("task %d: %s (exit %d)", id, task.Status, task.ExitCode)
		}
		select {
		case <-cli.ctx.Done():
			return cli.ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (cli *app) remotePresets(action string, args []string, kind, file string) error {
	if kind == "download" {
		data, err := cli.request("GET", "/api/config", nil)
		if err != nil {
			return err
		}
		var cfg core.Config
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		local := *cli
		local.cfg = cfg
		local.server = ""
		if action == "list" || action == "show" {
			return local.presets(action, args, kind, file)
		}
		if len(args) != 1 {
			return usage("presets save|delete ID --kind download")
		}
		id := args[0]
		if action == "save" {
			if file == "" {
				return usage("presets save requires --file")
			}
			data, err := cli.read(file)
			if err != nil {
				return err
			}
			var preset core.DownloadPreset
			if err := json.Unmarshal(data, &preset); err != nil {
				return err
			}
			preset.ID = id
			if preset.Name == "" {
				preset.Name = id
			}
			if preset.Fields == nil {
				return usage("download preset requires fields")
			}
			found := false
			for index, existing := range cfg.DownloadPresets {
				if existing.ID == id {
					cfg.DownloadPresets[index] = preset
					found = true
					break
				}
			}
			if !found {
				cfg.DownloadPresets = append(cfg.DownloadPresets, preset)
			}
		} else if action == "delete" {
			found := false
			var kept []core.DownloadPreset
			for _, preset := range cfg.DownloadPresets {
				if preset.ID == id {
					found = true
				} else {
					kept = append(kept, preset)
				}
			}
			if !found {
				return usage("unknown preset")
			}
			cfg.DownloadPresets = kept
		} else {
			return usage("presets list|show|save|delete")
		}
		if cli.dry {
			return cli.emit(cfg.DownloadPresets)
		}
		return cli.remote("PUT", "/api/config", cfg)
	}
	if kind != "media" {
		return usage("remote presets supports --kind media or download; built-in profiles are available locally")
	}
	if action == "list" {
		return cli.remote("GET", "/api/pipeline/presets", nil)
	}
	if len(args) != 1 {
		return usage("presets show|save|delete ID")
	}
	id := args[0]
	if action == "show" {
		data, err := cli.request("GET", "/api/pipeline/presets", nil)
		if err != nil {
			return err
		}
		var response struct {
			Presets []core.MediaPreset `json:"presets"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return err
		}
		for _, preset := range response.Presets {
			if preset.ID == id {
				return cli.emit(preset)
			}
		}
		return usage("unknown preset")
	}
	if action == "delete" {
		if cli.dry {
			return cli.emit(map[string]any{"delete": id})
		}
		return cli.remote("DELETE", "/api/pipeline/presets/"+urlPath(id), nil)
	}
	if action == "save" {
		if file == "" {
			return usage("presets save requires --file")
		}
		data, err := cli.read(file)
		if err != nil {
			return err
		}
		var preset core.MediaPreset
		if err := json.Unmarshal(data, &preset); err != nil {
			return err
		}
		preset.ID = id
		if preset.Name == "" {
			preset.Name = id
		}
		for _, operation := range preset.Operations {
			var options map[string]any
			if err := json.Unmarshal(operation.Config, &options); err != nil {
				return err
			}
			if _, err := strictOperation(operation.Type, options); err != nil {
				return err
			}
		}
		if cli.dry {
			return cli.emit(preset)
		}
		return cli.remote("POST", "/api/pipeline/presets", preset)
	}
	return usage("presets list|show|save|delete")
}
