# gmail-tools

A golang CLI tool for managing Gmail filters and messages. Builds into the `gmailcli` binary.

## Features
### Message Searching/Modification
`gmailcli search` provides an array of search tools which are not available through the Gmail interface, including custom "interest" categorization.

Set up a configuration file in ~/.gmailcli/config.yaml, and search for "interesting"
and "uninteresting" messages, and apply labels to matching messages. This can enable you to perform more powerful filtering on your inbox. An example exists in config_example.yaml

#### Search Plugins
The tools is enabled with an expanding set of plugin interfaces, which can be used for more complicated categorization, such as building on the "interest" categorization, classifying "out of date" messages, etc.

More to come, for more general purpose plugin interfaces.

Examples, via built-in plugins, are provided in the plugins directory.

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

### Command Aliases
As a convenience feature, command aliases can be specified in config.yaml under the
Aliases section. Each entry consists of the alias name and a command to run.

Aliases MAY reference each other (though there is a maximum depth enforced)

Positional arguments passed to the alias can be placed in a specific location in the
target command by using $n (starting at $1). Note that positional arguments MUST
either come before any flags or after the -- marker, since at alias parse time,
the semantics of the flags in the target command are not yet known.

The remainder of the arguments (not specified explicitly via $n) can also be placed
with $R. This defaults to the end of the command argument list.

## Set up/Development
### Building
```
make getdeps
make
make test

export PATH=$PATH:$(pwd)/bld
gmailcli ...
```

### API/Auth Setup
#### Getting an API key
This application does not provide a global API key. You will need to create an API project in the Google developer console.
1. Go to https://console.developers.google.com
2. Create a new project
3. Go to the Credential section in the new project
4. Click "Create credentials", and select OAuth client ID
5. Select type "Other" for the credential
6. Click the "Download JSON" button on the new credential.
7. Move the downloaded credential file to `~/.gmailcli/client_secret.json`

#### Logging in
To log into a gmail account, run any gmailcli command which attempts to access the service. Follow the link, and paste the authentication code into the console.

`gmailcli authorize` is conveniently provided which does nothing but sign in.

### Automation JSON interface (v1)

Use `--assert-email user@example.com` on reads and writes to verify the account
before message access. Authenticate interactively with `authorize` first.
JSON reads reuse modify credentials but never modify Gmail and bypass caches
and local interest plugins.

- `show ID --json --format metadata|full` returns a Gmail message resource.
- `show ID --thread [--json] [--format metadata|full]` fetches the entire
  conversation.
- `show ID --json --raw` returns the message with base64url RFC 5322 `raw`.
  Raw cannot be combined with thread or an explicit format. JSON cannot touch
  messages or use display flags.
- `labels list --json` returns an array of existing labels with exact `id`,
  `name`, and `type`. Slash-separated names do not imply membership.
  Create missing labels in Gmail's web UI; the CLI never creates labels.
- `account --json` returns the authenticated Gmail profile.
- `search --json --label-id Label_123 --include-spam-trash` enumerates pages
  automatically. Ordinary JSON retains its array schema (`messageId`,
  `threadId`, `timestamp`, `subject`, `from`, `labels`); metadata requires
  one GET per message plus label lookup. `--json --ids-only` skips these
  lookups and returns Gmail message references with `id` and `threadId`.
- `search --json --page-size 500 [--page-token TOKEN]` returns one unfiltered
  listing page as `{"messages":[],"nextPageToken":"..."}`. The token is omitted
  on the last page. Follow every token and deduplicate IDs. Paging rejects
  `--max`; JSON search rejects local filters, summaries, and mutations.
  `--label-id` is repeatable (all labels must match). Spam/trash inclusion is
  opt-in. Empty search succeeds with an empty array.

Resource JSON follows Gmail shapes: threads have `id` and `messages`;
messages have `id`, `threadId`, `labelIds`, `internalDate`, `payload`,
and `raw` where applicable. Optional empty resource arrays/fields are omitted.
Treat absent arrays as empty. `internalDate` is a decimal **string** of Unix
milliseconds, as in Gmail; legacy search `timestamp` is a JSON number in
milliseconds. Consumers should tolerate additional fields.

Successful JSON writes only the result to stdout. Errors exit nonzero and
write `{"error":{"kind":"invalid_input","message":"..."}}` to stderr.
HTTP failures include numeric `httpStatus`; kinds include `not_found` (404),
`authentication` (401/403), `throttled` (429), `http`, and `network`.
Exact-message label updates resolve all labels and check message existence
before mutation; server failures still cannot guarantee atomic multi-batch writes.

`update-msgs` checks message IDs using HTTP batches of up to 50 requests before
applying changes. Scripts with trusted message IDs can use `--skip-msg-checks`
to omit those checks (label names are still validated). Gmail may silently
ignore nonexistent IDs when this option is used. For example:

```sh
gmailcli update-msgs --skip-msg-checks -y --add-label xCliTouched MSG_ID...
```

Use `--thread` to treat all arguments as thread IDs and update every message in
those threads. All threads are fetched before any changes, and message ID checks
are automatically skipped because Gmail supplies the IDs:

```sh
gmailcli update-msgs --thread -y --add-label xCliTouched THREAD_ID...
```
