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
