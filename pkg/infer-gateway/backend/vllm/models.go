package vllm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
)

type Model struct {
	ID string `json:"id"`
}

type ModelList struct {
	Data []Model `json:"data"`
}

func (engine *vllmEngine) GetPodModels(pod *corev1.Pod) ([]string, error) {
	url := fmt.Sprintf("http://%s:%d/v1/models", pod.Status.PodIP, engine.MetricPort)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var modelList ModelList
	err = json.Unmarshal(body, &modelList)
	if err != nil {
		return nil, err
	}

	models := []string{}
	for _, model := range modelList.Data {
		models = append(models, model.ID)
	}
	return models, nil
}
