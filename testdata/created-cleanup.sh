#!/usr/bin/env bash

kubectl delete -n kyma-system authorizationpolicies.security.istio.io tracing-jaeger
kubectl delete -n kyma-system clusterrolebindings.rbac.authorization.k8s.io cluster-essentials-pod-preset-webhook
kubectl delete -n kyma-system configmaps tracing-grafana-dashboard
kubectl delete -n kyma-system podsecuritypolicies.policy 002-kyma-privileged
kubectl delete -n kyma-system servicemonitors.monitoring.coreos.com tracing-jaeger-operator
