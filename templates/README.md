# Templates

Each folder here is a template the ServFlow dashboard installs: a
`manifest.yaml` naming its inputs, required integrations, and the config files
it installs, plus those files.

The dashboard reads templates from `main`, so a change merged here reaches
every install without a release. Keep `main` installable: change a manifest
and the config files it names in the same pull request.
