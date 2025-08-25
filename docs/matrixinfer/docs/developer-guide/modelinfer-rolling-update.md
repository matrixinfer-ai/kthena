# Rollout Strategy

Rolling updates represent a critical operational strategy for online services aiming to achieve zero downtime. In the context of LLM inference services, the implementation of rolling updates is important to reduce the risk of service unavailability.

Currently, `ModelInfer` supports rolling upgrades at the `InferGroup` level, enabling users to configure `Partitions` to control the rolling process.

- Partition: Indicates the ordinal at which the `ModelInfer` should be partitioned for updates. During rolling update, all inferGroups from ordinal Replicas-1 to Partition are updated. All inferGroups from ordinal Partition-1 to 0 remain untouched.

Here’s a ModelInfer configured with rollout strategy:

```yaml
spec:
  rolloutStrategy:
    type: InferGroupRollingUpdate
    rollingUpdateConfiguration:
      partition: 0
```

In the following we’ll show how rolling update processes for a `ModelInfer` with four replicas. Three Replica status are simulated here:

- ✅ Replica has been updated
- ❎ Replica hasn’t been updated
- ⏳ Replica is in rolling update

|        | R-0 | R-1 | R-2 | R-3 | Note                                                                                          |
|--------|-----|-----|-----|-----|-----------------------------------------------------------------------------------------------|
| Stage1 | ✅   | ✅   | ✅   | ✅   | Before rolling update                                                                         |
| Stage2 | ❎   | ❎   | ❎   | ⏳   | Rolling update started, delete and recreate the replica with the largest sequence number      |
| Stage3 | ❎   | ❎   | ⏳   | ✅   | When the replica with the largest serial number is updated, the next replica will be updated. |
| Stage4 | ❎   | ⏳   | ✅   | ✅   | Update the next replica                                                                       |
| Stage5 | ⏳   | ✅   | ✅   | ✅   | Update the last replica                                                                       |
| Stage6 | ✅   | ✅   | ✅   | ✅   | Update completed                                                                              |

During a rolling upgrade, the controller deletes and rebuilds the replica with the highest sequence number among the replicas need to be updated. The next replica will not be updated until the new replica is running normally.