## kthena describe

Show detailed information about a specific resource

### Synopsis

Show detailed information about a specific resource.

You can describe templates and other kthena resources.

Examples:
  kthena describe template deepseek-r1-distill-llama-8b
  kthena describe model my-model
  kthena describe modelinfer my-inference
  kthena describe autoscaling-policy my-policy

### Options

```
  -h, --help               help for describe
  -n, --namespace string   Kubernetes namespace (default: current context namespace)
```

### SEE ALSO

* [kthena](kthena.md)	 - Kthena CLI for managing AI inference workloads
* [kthena describe autoscaling-policy](kthena_describe_autoscaling-policy.md)	 - Show detailed information about an autoscaling policy
* [kthena describe model](kthena_describe_model.md)	 - Show detailed information about a model
* [kthena describe modelinfer](kthena_describe_modelinfer.md)	 - Show detailed information about a model inference workload
* [kthena describe template](kthena_describe_template.md)	 - Show detailed information about a template

