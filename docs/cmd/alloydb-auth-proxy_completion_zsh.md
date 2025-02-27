## alloydb-auth-proxy completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(alloydb-auth-proxy completion zsh)

To load completions for every new session, execute once:

#### Linux:

	alloydb-auth-proxy completion zsh > "${fpath[1]}/_alloydb-auth-proxy"

#### macOS:

	alloydb-auth-proxy completion zsh > $(brew --prefix)/share/zsh/site-functions/_alloydb-auth-proxy

You will need to start a new shell for this setup to take effect.


```
alloydb-auth-proxy completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --http-address string   Address for Prometheus and health check server (default "localhost")
      --http-port string      Port for the Prometheus server to use (default "9090")
      --quiet                 Log error messages only
```

### SEE ALSO

* [alloydb-auth-proxy completion](alloydb-auth-proxy_completion.md)	 - Generate the autocompletion script for the specified shell

