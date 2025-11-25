# Developing

Welcome to the Stellar Go monorepo. These instructions help launch 🚀 you into making and testing code changes to this repository.

For details about what's in this repository and how it is organized read the [README.md](README.md).

If you're aiming to submit a contribution make sure to also read the [contributing guidelines](CONTRIBUTING.md).

## Requirements
To checkout, build, and run most tests these tools are required:
- Git
- [Go](https://golang.org/dl)

To run some tests these tools are also required:
- PostgreSQL 12+ server running locally, or set [environment variables](https://www.postgresql.org/docs/12/libpq-envars.html) (e.g. `PGHOST`, etc) for alternative host.
- MySQL 10.1+ server running locally.

## Get the code

Check the code out anywhere, using a `GOPATH` is not required.

```
git clone https://github.com/stellar/go-stellar-sdk
```

## Running tests

```
go test ./...
```

## Running tools

```
go run ./tools/<tool>
```

## Managing dependencies

* Supported on the two latest major Go releases
* Uses [Go Modules](https://github.com/golang/go/wiki/Modules) for dependency management (see [go.mod](./go.mod))
* Standard go build, go test, and go run workflows apply

### Adding/Removing dependencies

Add new dependencies by adding the import paths to the code. The next time you execute a Go command the tool will update the `go.mod` and `go.sum` files.

To add a specific version of a dependency use `go get`:

```
go get <importpath>@<version>
```

Before opening a PR make sure to run following command to tidy the module file. It will keep the go.* files tidy:
- `go mod tidy`

### Updating a dependency

Update an existing dependency by using `go get`:

```
go get <importpath>@<version>
```
Before opening a PR make sure to run these commands to tidy the module files:
 ```
 go mod tidy
 ```

Note: `go list -m all` may show that the dependency is still being used. It will be possible that the dependency is still an indirect dependency. If it's important to understand why the dependency is still being used, use `go mod why <importpath>/...` and `go mod graph | grep <importpath>` to understand which modules are importing it.

### Reviewing changes in dependencies

When updating or adding dependencies it's critical that we review what the
changes are in those dependencies that we are introducing into our builds. When
dependencies change the diff for the `go.mod` file may be complex to
understand. In that situation check each new or upgraded dependency,
and check each dependencies code diffs to see what is being imported.
Always treat code being imported as code written that needs review.

```
git checkout master
go list -m -json all > go.list.master
git checkout <branch>
golistcmp go.list.master <(go list -m -json all)
```

## Package source layout

While much of the code in individual packages is organized based upon different developers' personal preferences, many of the packages follow a simple convention for organizing the declarations inside of a package that aim to aid in your ability to find code.

In each package, there may be one or more of a set of common files:

- *errors.go*: This file should contains declarations (both types and vars) for errors that are used by the package.
- *example_test.go*: This file should contains example tests, as described at https://blog.golang.org/examples.
- *main.go/internal.go* (**deprecated**): Older packages may have a `main.go` (public symbols) or `internal.go` (private symbols).  These files contain, respectively, the exported and unexported vars, consts, types and funcs for the package. New packages do not follow this pattern, and instead follow the standard Go convention to co-locate structs and their methods in the same files. 
- *main.go* (**new convention**): If present, this file contains a `main` function as part of an executable `main` package.

In addition to the above files, a package often has files that contains code that is specific to one declared type.  This file uses the snake case form of the type name (for example `loggly_hook.go` would correspond to the type `LogglyHook`).  This file should contain method declarations, interface implementation assertions and any other declarations that are tied solely to that type.

Each non-test file can have a test counterpart like normal, whose name ends with `_test.go`.  The common files described above also have their own test counterparts... for example `internal_test.go` should contains tests that test unexported behavior and more commonly test helpers that are unexported.

Generally, file contents are sorted by exported/unexported, then declaration type  (ordered as consts, vars, types, then funcs), then finally alphabetically.

### Test helpers

Often, we provide test packages that aid in the creation of tests that interact with our other packages.  For example, the `support/db` package has the `support/db/dbtest` package underneath it that contains elements that make it easier to test code that accesses a SQL database.  We've found that this pattern of having a separate test package maximizes flexibility and simplifies package dependencies.


