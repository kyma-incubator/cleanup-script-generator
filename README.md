# Clean-up Script Generator

## Overview (mandatory)

Small program that compares manifests files created by Kyma CLI dry-runs to identify orphaned resources after upgrading Kyma to higher versions.
Can optionally create a bash script with kubectl deletion commands to remove these resources.

## Usage

Supports three arguments to compare dry-run manifests of Kyma installations.
1. Path to the first manifests file of the the installed version
2. Path to the second manifests file of the upgrade version
3. Optional: Name for the deletion script to be generated. If used creates a script that deletes orphaned resources
