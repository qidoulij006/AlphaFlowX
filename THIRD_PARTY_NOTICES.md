# Third-Party Notices

This project is distributed under the GNU Affero General Public License
version 3. See `LICENSE` for the full license text.

This notice summarizes third-party dependency license findings from the
current source tree. The authoritative dependency manifests are `go.mod`,
`go.sum`, `web/package.json`, and `web/package-lock.json`.

## Frontend Production Dependencies

The frontend production dependency scan was run with:

```bash
cd web
npx --yes license-checker --production --summary
```

Summary:

| License | Count |
| --- | ---: |
| MIT | 101 |
| ISC | 12 |
| Apache-2.0 | 2 |
| BSD-3-Clause | 2 |
| 0BSD | 1 |
| MIT AND ISC | 1 |

No GPL, LGPL, or AGPL frontend production dependency was found in this scan.

## Go Dependencies

The Go dependency review was based on:

```bash
go list -m -json all
go mod why -m <module>
```

Most Go dependencies are permissively licensed under MIT, Apache-2.0, BSD,
ISC, or similar licenses.

The notable copyleft dependency identified in the current dependency graph is:

| Module | Version | License | Current Use Path |
| --- | --- | --- | --- |
| `github.com/ethereum/go-ethereum` | `v1.16.7` | LGPL-3.0 | `nofx/trader/aster` -> `github.com/ethereum/go-ethereum/accounts/abi` |

`go-ethereum` is licensed under the GNU Lesser General Public License version
3. The project source includes the corresponding application source code. If
compiled binaries or container images are distributed separately, preserve the
LGPL notices and provide the applicable source and relinking rights required by
the LGPL.

The following modules were reviewed because a simple keyword classifier could
flag them as copyleft or unknown, but they were not blocking findings:

| Module | Finding |
| --- | --- |
| `github.com/hashicorp/hcl` | MPL-2.0; not needed by the current main module according to `go mod why`. |
| `github.com/hashicorp/go-bexpr` | MPL-2.0; not needed by the current main module according to `go mod why`. |
| `github.com/status-im/keycard-go` | MPL-2.0; not needed by the current main module according to `go mod why`. |
| `gopkg.in/yaml.v1` | Not needed by the current main module according to `go mod why`. |
| `github.com/gorilla/websocket` | BSD-style license. |
| `github.com/pkg/errors` | BSD-style license. |
| `github.com/vmihailenco/msgpack/v5` | BSD-style license. |
| `gopkg.in/dnaeon/go-vcr.v4` | BSD-style license. |

## Maintenance Requirement

Re-run dependency license checks whenever dependencies are added, removed, or
upgraded, and update this file before publishing the corresponding source code.
