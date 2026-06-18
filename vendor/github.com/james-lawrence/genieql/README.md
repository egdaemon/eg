# genieql - sql query and code generation.
its purpose is to generate a decent amount of the
boilerplate code for interacting with database/sql
as possible without putting any runtime dependencies
into your codebase. primary areas of focus are:
1. data scanners (hydrating structures from queries)
2. make support and maintaince for simple queries a breeze.
3. integrate well with the broader ecosystem. aka: scanners should play well
with query builders.

# is it production ready?
its nearing production ready, currently we have 1 more change we want to make to the dsl.
but has otherwise been stable for a few years.

# documentation
release notes, and roadmap documentation
can be found in the documentation directory.
everything else will be found in godoc.

## genieql commands
- genieql bootstrap - setups dialect information for generation from database connection strings.
- genieql auto - runs the gql scripts to generate database code.

## examples
see the examples directory.