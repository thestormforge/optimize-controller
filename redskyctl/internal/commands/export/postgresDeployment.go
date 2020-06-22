package export

// TODO temporary hack to allow offline testing
var pgDeployment = []byte(`{
    "apiVersion": "apps/v1",
    "kind": "Deployment",
    "metadata": {
        "annotations": {
            "deployment.kubernetes.io/revision": "2",
            "kubectl.kubernetes.io/last-applied-configuration": "{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"annotations\":{},\"labels\":{\"component\":\"postgres\"},\"name\":\"postgres\",\"namespace\":\"default\"},\"spec\":{\"selector\":{\"matchLabels\":{\"component\":\"postgres\"}},\"strategy\":{\"type\":\"Recreate\"},\"template\":{\"metadata\":{\"labels\":{\"component\":\"postgres\"}},\"spec\":{\"containers\":[{\"env\":[{\"name\":\"PGDATA\",\"value\":\"/var/lib/postgresql/data/pgdata\"},{\"name\":\"POSTGRES_DB\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_DATABASE\",\"name\":\"postgres-secret\"}}},{\"name\":\"POSTGRES_USER\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_USERNAME\",\"name\":\"postgres-secret\"}}},{\"name\":\"POSTGRES_PASSWORD\",\"valueFrom\":{\"secretKeyRef\":{\"key\":\"PG_PASSWORD\",\"name\":\"postgres-secret\"}}}],\"image\":\"postgres:11.1-alpine\",\"livenessProbe\":{\"exec\":{\"command\":[\"pg_isready\",\"-h\",\"localhost\",\"-U\",\"test_user\",\"-d\",\"test_db\"]},\"initialDelaySeconds\":10,\"periodSeconds\":5},\"name\":\"postgres\",\"ports\":[{\"containerPort\":5432,\"name\":\"postgres\"}],\"readinessProbe\":{\"initialDelaySeconds\":15,\"periodSeconds\":5,\"tcpSocket\":{\"port\":5432}},\"resources\":{\"limits\":{\"cpu\":1,\"memory\":\"2000Mi\"},\"requests\":{\"cpu\":1,\"memory\":\"2000Mi\"}},\"securityContext\":{\"allowPrivilegeEscalation\":false,\"runAsUser\":70},\"volumeMounts\":[{\"mountPath\":\"/var/lib/postgresql/data\",\"name\":\"postgres-storage\"}]}],\"volumes\":[{\"emptyDir\":{},\"name\":\"postgres-storage\"}]}}}}\n"
        },
        "creationTimestamp": "2020-06-17T19:22:59Z",
        "generation": 2,
        "labels": {
            "component": "postgres"
        },
        "name": "postgres",
        "namespace": "default",
        "resourceVersion": "955",
        "selfLink": "/apis/apps/v1/namespaces/default/deployments/postgres",
        "uid": "a695754f-c17b-406f-b9a6-5bc74890b2ea"
    },
    "spec": {
        "progressDeadlineSeconds": 600,
        "replicas": 1,
        "revisionHistoryLimit": 10,
        "selector": {
            "matchLabels": {
                "component": "postgres"
            }
        },
        "strategy": {
            "type": "Recreate"
        },
        "template": {
            "metadata": {
                "creationTimestamp": null,
                "labels": {
                    "component": "postgres"
                }
            },
            "spec": {
                "containers": [
                    {
                        "env": [
                            {
                                "name": "PGDATA",
                                "value": "/var/lib/postgresql/data/pgdata"
                            },
                            {
                                "name": "POSTGRES_DB",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_DATABASE",
                                        "name": "postgres-secret"
                                    }
                                }
                            },
                            {
                                "name": "POSTGRES_USER",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_USERNAME",
                                        "name": "postgres-secret"
                                    }
                                }
                            },
                            {
                                "name": "POSTGRES_PASSWORD",
                                "valueFrom": {
                                    "secretKeyRef": {
                                        "key": "PG_PASSWORD",
                                        "name": "postgres-secret"
                                    }
                                }
                            }
                        ],
                        "image": "postgres:11.1-alpine",
                        "imagePullPolicy": "IfNotPresent",
                        "livenessProbe": {
                            "exec": {
                                "command": [
                                    "pg_isready",
                                    "-h",
                                    "localhost",
                                    "-U",
                                    "test_user",
                                    "-d",
                                    "test_db"
                                ]
                            },
                            "failureThreshold": 3,
                            "initialDelaySeconds": 10,
                            "periodSeconds": 5,
                            "successThreshold": 1,
                            "timeoutSeconds": 1
                        },
                        "name": "postgres",
                        "ports": [
                            {
                                "containerPort": 5432,
                                "name": "postgres",
                                "protocol": "TCP"
                            }
                        ],
                        "readinessProbe": {
                            "failureThreshold": 3,
                            "initialDelaySeconds": 15,
                            "periodSeconds": 5,
                            "successThreshold": 1,
                            "tcpSocket": {
                                "port": 5432
                            },
                            "timeoutSeconds": 1
                        },
                        "resources": {
                            "limits": {
                                "cpu": "424m",
                                "memory": "981Mi"
                            },
                            "requests": {
                                "cpu": "424m",
                                "memory": "981Mi"
                            }
                        },
                        "securityContext": {
                            "allowPrivilegeEscalation": false,
                            "runAsUser": 70
                        },
                        "terminationMessagePath": "/dev/termination-log",
                        "terminationMessagePolicy": "File",
                        "volumeMounts": [
                            {
                                "mountPath": "/var/lib/postgresql/data",
                                "name": "postgres-storage"
                            }
                        ]
                    }
                ],
                "dnsPolicy": "ClusterFirst",
                "restartPolicy": "Always",
                "schedulerName": "default-scheduler",
                "securityContext": {},
                "terminationGracePeriodSeconds": 30,
                "volumes": [
                    {
                        "emptyDir": {},
                        "name": "postgres-storage"
                    }
                ]
            }
        }
    },
    "status": {
        "availableReplicas": 1,
        "conditions": [
            {
                "lastTransitionTime": "2020-06-17T19:24:27Z",
                "lastUpdateTime": "2020-06-17T19:24:27Z",
                "message": "Deployment has minimum availability.",
                "reason": "MinimumReplicasAvailable",
                "status": "True",
                "type": "Available"
            },
            {
                "lastTransitionTime": "2020-06-17T19:22:59Z",
                "lastUpdateTime": "2020-06-17T19:24:27Z",
                "message": "ReplicaSet \"postgres-679df96c5f\" has successfully progressed.",
                "reason": "NewReplicaSetAvailable",
                "status": "True",
                "type": "Progressing"
            }
        ],
        "observedGeneration": 2,
        "readyReplicas": 1,
        "replicas": 1,
        "updatedReplicas": 1
    }
}`)
