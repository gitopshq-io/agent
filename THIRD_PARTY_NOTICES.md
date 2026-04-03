# Third-Party Notices

This project is licensed under Apache-2.0.  
It includes third-party software under their respective licenses.

## Primary Direct Dependencies (from `go.mod`)

| Module | License |
| --- | --- |
| `github.com/go-git/go-git/v5` | Apache-2.0 |
| `google.golang.org/grpc` | Apache-2.0 |
| `gopkg.in/yaml.v3` | Apache-2.0 |
| `helm.sh/helm/v3` | Apache-2.0 |
| `k8s.io/api` | Apache-2.0 |
| `k8s.io/apimachinery` | Apache-2.0 |
| `k8s.io/client-go` | Apache-2.0 |
| `sigs.k8s.io/kustomize/api` | Apache-2.0 |

## Notable Transitive License Families

The dependency graph includes MPL-2.0 licensed modules (weak copyleft), including:

- `github.com/cyphar/filepath-securejoin`
- `github.com/hashicorp/errwrap`
- `github.com/hashicorp/go-multierror`

## SBOM

Release artifacts include a CycloneDX SBOM (`sbom-source.cdx.json`) for machine-readable dependency tracking.

