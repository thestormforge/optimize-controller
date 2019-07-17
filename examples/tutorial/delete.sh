kubectl delete trials --all
kubectl delete experiments --all
kubectl delete poddisruptionbudgets logstash
kubectl delete configmaps logstash-files logstash-patterns logstash-pipeline elasticsearch elasticsearch-test filebeat prometheus-config logstash monitoring
kubectl delete deployments --all
kubectl delete statefulsets --all
kubectl delete serviceaccount elasticsearch-client elasticsearch-data elasticsearch-master logstash
kubectl delete service elasticsearch-client elasticsearch-discovery elasticsearch-exporter logstash prometheus-config cost-model kube-state-metrics prometheus node-exporter
kubectl delete daemonset.apps/node-exporter
kubectl delete --all pods --namespace=default
