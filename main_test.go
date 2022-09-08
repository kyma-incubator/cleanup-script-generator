package main

import (
	"bytes"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCLI(t *testing.T) {
	tests := []struct {
		summary        string
		fromFile       string
		toFile         string
		outputFile     string
		ignored        string
		expectedOutput string
	}{
		{
			summary:    "same manifest",
			fromFile:   path.Join("testdata", "kyma-1.yaml"),
			toFile:     path.Join("testdata", "kyma-1.yaml"),
			outputFile: path.Join("testdata", "test-result.sh"),
		},
		{
			summary:    "two orphans after upgrade",
			fromFile:   path.Join("testdata", "kyma-1.yaml"),
			toFile:     path.Join("testdata", "kyma-2.yaml"),
			outputFile: path.Join("testdata", "test-result.sh"),
			expectedOutput: `#!/usr/bin/env bash

kubectl delete -n kyma-system authorizationpolicies.security.istio.io tracing-jaeger
kubectl delete -n kyma-system clusterrolebindings.rbac.authorization.k8s.io cluster-essentials-pod-preset-webhook
kubectl delete -n kyma-system configmaps tracing-grafana-dashboard
kubectl delete -n kyma-system podsecuritypolicies.policy 002-kyma-privileged
kubectl delete -n kyma-system servicemonitors.monitoring.coreos.com tracing-jaeger-operator
`,
		},
		{
			summary:    "one orphans after upgrade with ignored",
			fromFile:   path.Join("testdata", "kyma-1.yaml"),
			toFile:     path.Join("testdata", "kyma-2.yaml"),
			outputFile: path.Join("testdata", "test-result.sh"),
			ignored:    "servicemonitor.monitoring.coreos.com:tracing-jaeger-operator,configmap:tracing-grafana-dashboard",
			expectedOutput: `#!/usr/bin/env bash

kubectl delete -n kyma-system authorizationpolicies.security.istio.io tracing-jaeger
kubectl delete -n kyma-system clusterrolebindings.rbac.authorization.k8s.io cluster-essentials-pod-preset-webhook
kubectl delete -n kyma-system podsecuritypolicies.policy 002-kyma-privileged
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.summary, func(t *testing.T) {
			buf := bytes.NewBufferString("")
			err := run(buf, flags{
				fromFile:   tc.fromFile,
				toFile:     tc.toFile,
				ignored:    tc.ignored,
				outputFile: tc.outputFile,
			})
			defer os.Remove(tc.outputFile)
			require.NoError(t, err)

			content, err := os.ReadFile(tc.outputFile)
			if tc.expectedOutput != "" {
				require.NoError(t, err)
				require.Equal(t, tc.expectedOutput, string(content))
			} else {
				require.Error(t, err)
			}
		})
	}
}
