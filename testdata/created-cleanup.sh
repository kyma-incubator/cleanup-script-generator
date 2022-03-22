#!/usr/bin/env bash

kubectl delete -n kyma-system servicemonitors.monitoring.coreos.com rafter-controller-manager
kubectl delete -n kyma-system deployments.apps rafter-asyncapi-svc
