```plantuml
actor operator
actor user

package "Control Plane" as control_plane {
	package "CRD" as crd {
		component "Model"
		component "ModelRoute"
		component "ModelServer"
		component "ModelInfer"
		component "AutoScalingPolicy"
	}
	
	package "Model (Registry) (main.go)" {
		package "Registry Controllers" {
			component "ModelController" as model_controller
		}
	}
	
	package "Infer Gateway (Network) (main.go)" {
		component "Model Router" as modelrouter {
			component "Rate Limiter" as rate_limiter
			component "scheduler" {
				component "Filter Plugins"
				component "Score Plugins"
				component "Pod Selector"
			}
			component "connector"
		}
		package "Network Controllers" {
			component "ModelRouteController" as modelroute_controller
			component "ModelServerController" as modelserver_controller
		}
		database "In-Memory Datastore" as network_datastore {
			component "Model Route" as modelroute_data
			component "Model Server"
			component "Pods"
		}
	}
	
	package "Infer Controller (Workload) (main.go)" {
		package "Workload Controllers" {
			component "ModelInferController" as modelinfer_controller {
				component "Replica Manager"
				component "Pod Lifecycle Manager"
				component "Rolling Update Manager"
				component "InferGroup Manager"
			}
		}
		database "In-Memory Datastore" as workload_datastore {
			component "InferGroups"
			component "Roles (Prefill/Decode)"
			component "Pod Status"
		}
	}
	
	package "Autoscaler (main.go)" {
		component "Optimizer" as autoscaler_optimizer {
			component "Metric Collector"
			component "Scaling Algorithm"
			component "Policy Engine"
		}
		component "AutoscalingPolicyController" as autoscaling_controller
	}
}

package "Data Plane" as data_plane {
	
	package "Inference Pods" as inference_pods {
		package "Prefill Pods" {
			component "Prefill Pod 1" as PP1 {
				[vLLM Engine]
				[KV Cache]
			}
			component "Prefill Pod N" as PPN {
				[vLLM Engine]
				[KV Cache]
			}
		}
		
		package "Decode Pods" {
			component "Decode Pod 1" as DP1 {
				[vLLM Engine]
				[KV Cache]
			}
			component "Decode Pod N" as DPN {
				[vLLM Engine]
				[KV Cache]
			}
		}
	}
	
	database "ETCD" as etcd {
		package "CR" {
			component "Model" as model_cr
			component "ModelRoute" as modelroute_cr
			component "ModelServer" as modelserver_cr
			component "ModelInfer" as modelinfer_cr
			component "AutoScalingPolicy" as autoscalingpolicy_cr
			
			modelroute_cr --> modelserver_cr : ref
	
	
		}
	}
}

control_plane ---down---> data_plane


operator --> crd : create/update/delete
user --> modelrouter : infer request

' datastore
modelroute_controller --> network_datastore : sync
modelserver_controller --> network_datastore : sync
modelrouter --> network_datastore : consume
modelroute_data --> rate_limiter : callback




```