# IronCore in a Box

[IronCore in a Box](https://github.com/ironcore-dev/ironcore-in-a-box) is the public local IronCore demo environment. Its README describes creating a kind cluster with make up. Creating that environment is separate from this application's read-only operation.

This integration targets an existing, explicitly selected local Kubernetes context. The default upstream stack may lack the four APIs needed by Fabric Explorer. The helper reports that condition rather than installing operators, CRDs or fixtures into your cluster.

## Run

1. Follow the upstream project's setup instructions in its own checkout.
2. Confirm your local context and use a kubeconfig stored outside this repository.
3. Build this application with make build.
4. Run the read-only preflight/helper:

~~~sh
./scripts/run-in-a-box.sh /path/to/local-reader.kubeconfig your-explicit-context
~~~

The fixed preflight queries read CRD declarations for switches.sonic.networking.metal.ironcore.dev, nodes.fabric.ironcore.dev, interfaces.fabric.ironcore.dev and cells.fabric.ironcore.dev. They do not read core Kubernetes Nodes or Secrets. A missing or forbidden CRD query stops the helper with a useful message. CRD discovery permission is needed by the helper only; the application itself needs list access to its four resources.

After preflight, the backend performs direct read-only API LIST requests and keeps partial/stale metadata. No SSH proxy, equipment endpoint, additional backend service or database is used. The helper never changes your current context or copies credentials.

For a pure demo with batteries included, run fixture mode or docker compose up --build. It includes synthetic two-spine/two-leaf inventory with Cell configuration and ports and needs no operators/cluster. Fixture data is not automatically installed into any cluster.

The optional deploy/deployment.yaml manifest is intended for explicit deployment by the operator into their selected environment. It is not applied by this project or by tests. Its ServiceAccount can get/list/watch only existing Switch, Fabric Node, Interface and Cell resources. Provide namespace-specific Cell roles where needed, using deploy/namespaced-cells.example.yaml.

## Limits

Kubernetes configuration is the only live data source in this distribution. The public default environment was checked through upstream documentation; a running IronCore in a Box cluster was not launched or modified as part of preparing this repository. Direct API behavior is covered by fake-client/local HTTP tests. Missing Fabric APIs must not be represented as a working physical fabric.
