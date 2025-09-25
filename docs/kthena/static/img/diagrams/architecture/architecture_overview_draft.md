# Architecture diagrams

## Reconcile
```plantuml
' Styling rules
skinparam component {
  BackgroundColor<<CR>> LightYellow
  BorderColor<<CR>> DarkGoldenRod
  BackgroundColor<<Controller>> LightBlue
  BorderColor<<Controller>> Navy
}

actor user

rectangle "reconcile" {
	' Style for CRs
	component "model_cr" <<CR>>
	component "autoscalingpolicy_cr" <<CR>>
	component "autoscalingpolicybinding_cr" <<CR>>
	component "modelinfer_cr" <<CR>>
	component "modelserver_cr" <<CR>>
	component "modelroute_cr" <<CR>>
	
	' Style for Controllers
	component "model_controller" <<Controller>>
	component "autoscaler_controller" <<Controller>>
	component "modelinfer_controller" <<Controller>>
	component "modelserver_controller" <<Controller>>
	component "modelroute_controller" <<Controller>>
}

rectangle "infer" {
	component "infer router" as infer_router
	component "inference pods" as pods
}


' Relationships

user --> model_cr

model_cr --> model_controller : inform
autoscalingpolicy_cr --> autoscaler_controller : inform
autoscalingpolicybinding_cr --> autoscaler_controller : inform
model_controller --> autoscalingpolicy_cr : update
model_controller --> autoscalingpolicybinding_cr : update
model_controller --> modelserver_cr : update
model_controller --> modelroute_cr : update
autoscaler_controller --> modelinfer_cr : update
modelinfer_cr --> modelinfer_controller : inform
modelserver_cr --> modelserver_controller : inform
modelroute_cr --> modelroute_controller : inform

modelserver_controller --> infer_router : update
modelroute_controller --> infer_router : update
modelinfer_controller --> pods : update
```

## Component
```plantuml
skinparam linetype polyline
skinparam component<<dashed>> {
  BorderStyle dashed
  BorderColor gray
  BackgroundColor white
}

actor operator
actor user

rectangle "Control Plane" as control_plane {
	component "k8s API Server" as k8s_api_server {
		component "CRD" as crd {
			component "Model" as model_crd
			component "ModelRoute" as modelroute_crd
			component "ModelServer" as modelserver_crd
			component "ModelInfer" as modelinfer_crd
			component "AutoScalingPolicy" as autoscalingpolicy_crd
			model_crd -[hidden]-> modelroute_crd
			model_crd -[hidden]-> modelinfer_crd
			model_crd -[hidden]-> autoscalingpolicy_crd
			modelroute_crd -[hidden]-> modelserver_crd
		}
	}
	
	rectangle "Network" as network {
		component "KthenaRouter Webhook (main.go)" as inferrouter_webhook
		rectangle "Infer Router (main.go)" {
			component "Model Router" as modelrouter {
				component "Auth" as auth
				component "Rate Limiter" as rate_limiter
				component "Fairness scheduling" as fairness_scheduling
				component "Load balance" as load_balance
				component "Router Proxy" as router_proxy
				component "Scheduler (schedule Pods)" as scheduler {
					component "Filter Plugins" {
						component "least-request"
						component "lora-affinity"
					}
					component "Score Plugins" {
						component "least-request"
						component "kv-cache"
						component "least-latency (TTFT & TPOT)"
						database "prefix-cache"
						component "gpu-cache"
						component "random"
					}
					component "PDGroup Scheduling"
				}
				auth --> rate_limiter
				rate_limiter --> fairness_scheduling
				fairness_scheduling --> load_balance
				load_balance --> router_proxy
				load_balance --> scheduler : schedule inference pods
			}
			rectangle "Network Controllers" {
				component "ModelRouteController" as modelroute_controller
				component "ModelServerController" as modelserver_controller
			}

			database "In-Memory Datastore" as network_datastore {
				component "Model Route" as modelroute_data
				component "Model Server" as modelserver_data
				component "Pods" as network_datastore_pods
				component "RequestWaitingQueue" as request_waiting_queue
			}
			fairness_scheduling --> request_waiting_queue : fairness
			router_proxy --> modelserver_data
			scheduler --> modelserver_data
			scheduler --> network_datastore_pods
		}
		' layout
		inferrouter_webhook -[hidden]-> modelrouter
	}


	rectangle "Registry" as registry {
		component "Registry Webhook (main.go)" as registry_webhook
		component "ModelController (main.go)" as model_controller
		component "AutoscalingPolicyController (main.go)" as autoscaling_controller {
			component "Optimizer" as autoscaler_optimizer {
				component "Metric Collector"
				component "Scaling Algorithm"
				component "Policy Engine"
			}
		}
		
		' layout
		registry_webhook -[hidden]-> model_controller
	}

	rectangle "Workload" {
		component "ModelInfer Webhook (main.go)" as modelinfer_webhook
		rectangle "Infer Controller (main.go)" as modelinfer_controller {
			database "Work Queue" as work_queue
			component "Worker" as worker {
				component "Manage Replica, Pod Lifecycle, Rolling Update, InferGroup"
			}
			
			database "In-Memory Datastore" as workload_datastore {
				component "InferGroups"
				component "Roles (Prefill/Decode)"
				component "Pod Status"
			}
			worker --> work_queue
			worker --> workload_datastore
		}
		
		' layout
		modelinfer_webhook -[hidden]-> modelinfer_controller
	}
}

rectangle "Data Plane" as data_plane {
	component "KV connector" as connector {
		component "HTTP"
		component "LMCache"
		component "MoonCake"
		component "Nixl"
	}
	component "Inference Pods" as inference_pods {
		component "Group 0" as g0 {
			node "Role A " as g0ra {
				component "Entry Pod" as g0rae
				component "Worker Pod" as g0raw
				component "downloader" <<dashed>> as g0rad
				component "runtime (metrics)" <<dashed>> as g0rar
			}
			node "Role B" as g0rb {
				component "Entry Pod" as g0rbe
				component "downloader" <<dashed>> as g0rbd
				component "runtime (metrics)" <<dashed>> as g0rbr
			}
			node "Role N" as g0rn {
				component "Entry Pod" as g0rne
				component "Worker Pod" as g0rnw
				component "downloader" <<dashed>> as g0rnd
				component "runtime (metrics)" <<dashed>> as g0rnr
			}
		}
		
		component "Group 1" as g1 {
			node "Role A " as g1ra {
				component "Entry Pod" as g1rae
				component "Worker Pod" as g1raw
				component "downloader" <<dashed>> as g1rad
				component "runtime (metrics)" <<dashed>> as g1rar
			}
			node "Role B" as g1rb  {
				component "Entry Pod" as g1rbe
				component "downloader" <<dashed>> as g1rbd
				component "runtime (metrics)" <<dashed>> as g1rbr
			}
			node "Role N" as g1rn  {
				component "Entry Pod" as g1rne
				component "Worker Pod" as g1rnw
				component "downloader" <<dashed>> as g1rnd
				component "runtime (metrics)" <<dashed>> as g1rnr
			}
		}
	}
}

' request
operator ----> crd : create/update/delete
user ----> auth : infer request
router_proxy ----down---> inference_pods
router_proxy --down--> connector : PD-Disaggregation mode
connector ----down---> inference_pods
network_datastore ------> inference_pods : periodically get metrics from backend (vLLM, sglang)
autoscaling_controller ------> inference_pods : periodically get metrics


' datastore
modelroute_controller --> network_datastore : sync
modelserver_controller --> network_datastore : sync
modelrouter --> network_datastore : consume
modelroute_data -up-> rate_limiter : callback


```