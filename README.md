# gmail-tools

A golang CLI tool for managing Gmail filters and messages. Builds into the `gmailcli` binary.

## Features
### Filters
Use the `filter` subcommand to perform actions on gmail filters.

#### Filter templating

Create "templates" in your filters, which can be copied from other filters that
define the primary version. Use the `filter update` command do do this.

Each template is marked with a meta tag in the following format:

`{(M3TA mytemplatename) "match against this stuff"}`

Primary templates use `M3TAP` instead.

#### Other filter features
- The `filter replace` command allows you do to do regex replacements on all filters.

### Message Searching
Set up a configuration file in ~/.gmailcli/config.yaml, and search for "interesting"
and "uninteresting" messages, and apply labels to matching messages. This can enable you to perform more powerful filtering on your inbox.

## Set up/Development
```
make getdeps
make
make test

export PATH=$PATH:$(pwd)/bld
gmailcli ...
```
