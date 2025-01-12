# INSTALL KeyPub

This document represents steps to deploy a local instance of keypub in your machine for testing and development purposes.

#### Create your config file
You can easily bootstrap by copying example config file.
```bash
$ cp config.json.example config.json
```

#### Create a hostkey
Note that if you enter passphrase when generating key, you should modify config file by adding `server.host_key_passphrase`.
```bash
$ ssh-keygen -f ./.host
```

#### Create database
You can also bootstrap here by copying the database generated for jet
```bash
$ make generate
$ cp internal/db/keysdb.sqlite3 .
```

#### Build and up using docker compose
```bash
$ docker compose build
$ docker compose up -d
```

#### Verify your setup
```bash
$ ssh localhost -p 8022 about
```
