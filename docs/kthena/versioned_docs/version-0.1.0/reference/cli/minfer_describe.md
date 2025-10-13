## minfer describe

Show detailed information about a specific resource

### Synopsis

Show detailed information about a specific resource.

You can describe templates and other kthena resources.

Examples:
  minfer describe template deepseek-r1-distill-llama-8b
  minfer describe model my-model
  minfer describe modelinfer my-inference
  minfer describe autoscaling-policy my-policy

### Options

```
  -h, --help               help for describe
  -n, --namespace string   Kubernetes namespace (default: current context namespace)
```

### SEE ALSO

* [minfer](minfer.md)	 - Kthena CLI for managing AI inference workloads
* [minfer describe autoscaling-policy](minfer_describe_autoscaling-policy.md)	 - Show detailed information about an autoscaling policy
* [minfer describe model](minfer_describe_model.md)	 - Show detailed information about a model
* [minfer describe modelinfer](minfer_describe_modelinfer.md)	 - Show detailed information about a model inference workload
* [minfer describe template](minfer_describe_template.md)	 - Show detailed information about a template

