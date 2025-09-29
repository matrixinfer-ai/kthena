# Benchmark for inference router


This tool is based on [`sglang/bench_serving.py`](https://github.com/sgl-project/sglang/blob/main/python/sglang/bench_serving.py). We have packaged it as a Docker image and made it runnable as a Kubernetes Job for easy benchmarking in a cluster environment.

### Usage Notes

- **Hugging Face Downloads:**  
  The tool downloads some model/configuration files from Hugging Face at runtime. If you are running in an environment without public internet access, you must pre-download these files and include them in your Docker image.

- **Tokenizer Parameter:**  
  - On a public network, you can set the `--tokenizer` parameter to the model name (e.g., `deepseek-ai/DeepSeek-R1-Distill-Qwen-7B`).
  - In a restricted network, set `--tokenizer` to the path of the pre-downloaded tokenizer.  
    For example, for `deepseek-ai/DeepSeek-R1-Distill-Qwen-7B`, use:  
    ```
    /root/.cache/huggingface/hub/models--deepseek-ai--DeepSeek-R1-Distill-Qwen-7B/snapshots/916b56a44061fd5cd7d6a8fb632557ed4f724f60/
    ```
  - How to pre-download tokenizer: Run the container on a public network using the model name as the `--tokenizer` parameter for testing. Once the test runs normally, the container will contain the tokenizer. At this point, commit the container to a new image, so that the image will include the corresponding tokenizer. 

- **HostPath Volume for Caching:**  
  We use a `hostPath` volume to cache data generated during benchmarking, especially for datasets like `"generated-shared-prefix"`. Generating test data based on the tokenizer can be time-consuming, but by caching the results on the host, subsequent runs are much faster. You can use any Kubernetes-supported storage type for caching, not just `hostPath`.
