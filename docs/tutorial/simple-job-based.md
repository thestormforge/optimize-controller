# Red Sky Tutorial

## Simple Job Based Experiment

For this experiment we will create an experiment where the application under test runs to completion from within the trial run job.

### Prerequisites

We will be using the Logstash benchmark CLI tool which is not included as part of the stock image. You can build the `benchmark-cli:6.7.0` image used in this example directly into your minikube cluster using the following commands:

```bash
$ eval $(minikube docker-env)
$ docker build -t benchmark-cli:6.7.0 - << 'EOF'
FROM gradle:4.10 as benchmark-cli
RUN git clone --depth 1 --branch v6.7.0 https://github.com/elastic/logstash.git \
  && gradle -p logstash/tools/benchmark-cli/ assemble

FROM docker.elastic.co/logstash/logstash-oss:6.7.0
COPY --from=benchmark-cli /home/gradle/logstash/tools/benchmark-cli/build/libs \
  /usr/share/logstash/tools/benchmark-cli/build/libs
ENV BATCH_SIZE=128 WORKERS=2 REPEAT_DATA=1 TESTCASE=baseline
ENTRYPOINT exec benchmark.sh --local-path $PWD --ls-batch-size $BATCH_SIZE --ls-workers $WORKERS --repeat-data $REPEAT_DATA --testcase $TESTCASE
EOF
```

### Application Setup

Since the trial run job will be the only process in this experiment, there are no workloads to deploy prior to running the experiment. However, in order to parameterize the trial run job, we must have a Kubernetes object that we can apply patches to. A `ConfigMap` can be used to supply environment variables to the `benchmark-cli` image used for the job:

```yml
apiVersion: v1
kind: ConfigMap
metadata:
  name: logstash-benchmark
data:
  BATCH_SIZE: "128"
  WORKERS: "2"
  REPEAT_DATA: "1"
  TESTCASE: "baseline"
```

### Experiment

The Logstash benchmark CLI tool allows for the parameterization of both the batch size and number of workers that are related to the performance of the system, those will become the parameters for this experiment.

In general, experiments can either vary the amount of work they perform or the amount of time that they run. For this experiment, the benchmark CLI produces a constant amount of work so we will optimize on the amount of time taken to perform that work.

The configuration map represents the application state which we have control over, the experiment will need to patch the configuration map, therefore impacting the environment of the trial run job.

```yml
apiVersion: redsky.carbonrelay.com/v1alpha1
kind: Experiment
metadata:
  name: logstash-benchmark
spec:
  parameters:
  - name: batchSize
    min: 128
    max: 1024
  - name: workers
    min: 1
    max: 10
  metrics:
  - name: time
    minimize: true
    query: "{{duration .StartTime .CompletionTime}}"
  template: # trial
    spec:
      template: # job
        spec:
          template: # pod
            spec:
              containers:
              - name: benchmark-cli
                image: benchmark-cli:6.7.0
                envFrom:
                - configMapRef:
                    name: logstash-benchmark
  patches:
  - type: json
    targetRef:
      kind: ConfigMap
      name: logstash-benchmark
    patch: |
      [
        { "op": "replace", "path": "/data/BATCH_SIZE", "value": "{{.Values.batchSize}}" },
        { "op": "replace", "path": "/data/WORKERS", "value": "{{.Values.workers}}" }
      ]
```

### Serverless Trials

This particular experiment is unlikely to produce interesting results using an external optimization platform, we can manually create trial instances in an attempt to understand the effect of the batch size and worker count on benchmark runtime.

```yml
apiVersion: redsky.carbonrelay.com/v1alpha1
kind: Trial
metadata:
  name: logstash-benchmark
spec:
  assignments:
    batchSize: 256
    workers: 2
```

Additional trials can be created in this manor and `kubectl` can be used to inspect the results. Note that you must set the experiment reference if you wish to change the name of each trial.

### Alternative Approaches

The `jobTemplate` field of the experiment allows full control over the specification of the job objects created for each trial run. Instead of building a custom image for the benchmark CLI, it would have been possible to build just the necessary JAR file and use the stock Logstash image.

To build a local copy of the `benchmark-cli.jar` build it in a local Docker image (do not run this in your minikube Docker environment), copy it back out to your host and mount it into your minikube cluster:

```bash
$ docker run --name benchmark-cli gradle:4.10 /bin/bash -c \
     'git clone --depth 1 --branch v6.7.0 https://github.com/elastic/logstash.git \
     && gradle -p logstash/tools/benchmark-cli/ assemble'
$ docker cp benchmark-cli:/home/gradle/logstash/tools/benchmark-cli/build/libs/benchmark-cli.jar .
$ docker rm benchmark-cli
$ minikube mount "$PWD:/benchmark"
```

Now you can alter the definition of your experiment to use the stock image by overriding the container command and arguments and mounting the JAR to the container:

```yml
apiVersion: redsky.carbonrelay.com/v1alpha1
kind: Experiment
metadata:
  name: logstash-benchmark
spec:
  parameters:
  - name: batchSize
    min: 128
    max: 1024
  - name: workers
    min: 1
    max: 10
  metrics:
  - name: t
    minimize: true
    query: "{{duration .Status.StartTime .Status.CompletionTime}}"
  jobTemplate:
    spec:
      template:
        spec:
          volumes:
          - name: benchmark-cli-libs
            hostPath:
              path: /benchmark
          containers:
          - name: benchmark
            image: docker.elastic.co/logstash/logstash-oss:6.7.0
            command: ["benchmark.sh"]
            args: [ "--local-path", ".",
              "--ls-batch-size", "$(BATCH_SIZE)",
              "--ls-workers", "$(WORKERS)",
              "--repeat-data", "$(REPEAT_DATA)",
              "--testcase", "$(TESTCASE)" ]
            envFrom:
            - configMapRef:
                name: logstash-benchmark
            volumeMounts:
            - name: benchmark-cli-libs
              mountPath: /usr/share/logstash/tools/benchmark-cli/build/libs
  patches:
  - type: json
    targetRef:
      kind: ConfigMap
      name: logstash-benchmark
    patch: |
      [
        { "op": "replace", "path": "/data/BATCH_SIZE", "value": "{{.Values.batchSize}}" },
        { "op": "replace", "path": "/data/WORKERS", "value": "{{.Values.workers}}" }
      ]
```

Yet another option would be to use an `emptyDir` volume and have an initialization container build the JAR file, however this approach can dramatically increase the overall time taken to run the experiment.

