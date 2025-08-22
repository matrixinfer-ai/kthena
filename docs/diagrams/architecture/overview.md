```plantuml
actor operator
actor user

package "Control Plane" {
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
			component "scheduler"
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
			component "ModelInferController" as modelinfer_controller
		}
		database "In-Memory Datastore" as workload_datastore
	}
}

package "Data Plane" {
	
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




operator --> crd : create/update/delete
user --> modelrouter : infer request

' reconcile
modelroute_controller ..> modelroute_cr : reconcile
modelserver_controller ..> modelserver_cr : reconcile
modelserver_controller ..> inference_pods : watch

' datastore
modelroute_controller --> network_datastore : sync
modelserver_controller --> network_datastore : sync
modelrouter --> network_datastore : consume
modelroute_data --> rate_limiter : callback




```