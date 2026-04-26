# Managing Config Management Plugins

`Instance.spec.forProvider.configManagementPlugins` manages Argo CD Config Management Plugins v2 for an Akuity Argo CD instance.

Use [examples/instance/config-management-plugins.yaml](../../examples/instance/config-management-plugins.yaml) as the focused example, or [examples/instance/detailed.yaml](../../examples/instance/detailed.yaml) for a broader instance configuration.

## Take Over Existing Plugins

If plugins already exist in Akuity, add their definitions to `configManagementPlugins` before applying the Crossplane managed resource. The provider owns the plugin map that appears in the managed resource spec, so missing entries may be treated as intentional desired state.

## Add Or Update A Plugin

Each map key is the plugin name. The value defines whether the plugin is enabled, the image, and the Config Management Plugin spec:

```yaml
configManagementPlugins:
  tanka:
    enabled: true
    image: grafana/tanka:0.25.0
    spec:
      version: "v1.0"
      discover:
        fileName: jsonnetfile.json
      init:
        command: ["jb", "update"]
      generate:
        command: ["sh", "-c"]
        args:
          - tk show environments/$PARAM_ENV --dangerous-allow-redirect
      parameters:
        static:
          - name: env
            required: true
            string: default
```

## Delete A Plugin

Remove that plugin key from `configManagementPlugins` and apply the resource. Review the diff carefully before applying changes to production instances.
