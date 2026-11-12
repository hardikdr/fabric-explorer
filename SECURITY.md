# Security

This application is read-only. Use only the documented get/list/watch permissions for its four Kubernetes resource types. It does not retrieve Secrets, execute remote commands, or modify resources or equipment.

Keep kubeconfigs, passwords, keys, operational captures and deployment-specific overrides outside this repository. Fixture/test data must be synthetic and screenshots must be generated from fixture mode. Docker builds must use this application's directory as their context.

Local development binds to loopback. The application has no built-in authentication; provide authenticated access at the deployment boundary before making a live inventory available to other users. Resource names, labels, conditions and network declarations may themselves be sensitive even though the API removes credential fields.

Report suspected security issues privately to the repository maintainers using GitHub's private vulnerability reporting when enabled. Do not include credentials or operational environment details in public issues.
