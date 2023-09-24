# vault-plugin-sidecar

Small sidecar container to pull custom vault plugins from storage, write them into a mounted pvc and register them with Vault

This is built with banzai's vault operator in mind, but ti should be generic enough to work with any Vault deployment.
The kubernetes [example](kubernetes/deployment.yaml) only works with Banzai, so if you differ you will have yo figure your own deployment.

## Storage
The sidecar can only pull from AWS S3, or s3 compatible storage, like Digital Ocean's Spaces.

Remote:
```yaml
s3:
  endpoint: "https://<bucket>.fra1.digitaloceanspaces.com"
  bucket: "<bucket>"
  token: "<secretName>"
  key: "<secretKey>"
```
Local
```yaml
vault:
  plugin-dir: "/vault/plugins"

```
## Plugins

Configuration for the plugins, it supports multiple architectures in case you build you plugins for other architectures.
In the example below `s3://s3provider.domain.com/bucket/plugin1-v3.0.0-amd64` will be downloaded and written as `/vault/plugins/plugin1`

The sidecar will then use the version and type from the config, and automatically calculate the sha256 hash.
It is on the todo list to host the sha256 sums in the bucket as well.

```yaml
plugins:
  plugin1: # <-- filename destination
    version: v3.0.0
    type: secret
    arch:
      amd64: plugin1-v3.0.0-amd64 # <-- filename source
      arm64: plugin1-v3.0.0-arm64 # <-- filename source
  plugin2: # <-- filename destination
    filename: plugin2-v1.2.1-amd64 # <-- filename source
    version: v1.2.1
    type: auth
    arch:
      amd64: plugin2-v1.2.1-amd64 # <-- filename source
      arm64: plugin2-v1.2.1-arm64 # <-- filename source
```

Environment Variables

- CONFIG            : path to the config yaml file, eg the mounted path `/etc/plugins.yaml`
- VAULT_ADDR        : the vault address, eg `https://vault.default:8200` With Banzai you can also use the mutating  webhook for this : https://bank-vaults.dev/docs/mutating-webhook/configuration/
- VAULT_SKIP_VERIFY : set to `true` if your vault tls cert is self-signed
- SA_NAME           : the Service Account name, this app only works by authenticating via kubernetes auth. If you create your own deployment, make sure the SA is registered in vault and has sudo permissions on the [catalog path](https://developer.hashicorp.com/vault/api-docs/system/plugins-catalog#register-plugin)
- ARCH              : defaults to `amd64`. To override this, set the ENV var.