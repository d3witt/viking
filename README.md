# Viking CLI 🗺️

### Simple way to manage your remote machines

Bare metal servers are awesome. They let you pick where to run your software and how to deploy it. You get full control to make the most of the server's resources. No limits, no compromises. That's real freedom.

Viking makes it easier to work with them.

```
NAME:
   viking - Manage your SSH keys and remote machines

USAGE:
   viking [global options] command [command options]

VERSION:
   v1.0

COMMANDS:
   key      Manage SSH keys
   machine  Manage your machines
   config   Get config directory path
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

## 🚀 Installation

See [releases](https://github.com/d3witt/viking/releases) for pre-built binaries.

On Unix:
```
env CGO_ENABLED=0 go install -ldflags="-s -w" github.com/d3witt/viking@latest
```

On Windows cmd:
```
set CGO_ENABLED=0
go install -ldflags="-s -w" github.com/d3witt/viking@latest
```

On Windows powershell:
```
$env:CGO_ENABLED = '0'
go install -ldflags="-s -w" github.com/d3witt/viking@latest
```

## 📄 Usage

#### 🛰️ Add machine:
```
$ viking machine add --name deathstar --key starkey 168.112.216.50
Machine deathstar added.
```
> [!NOTE]  
> The key flag is not required. If a key is not specified, SSH Agent will be used to connect to the server.

#### 📡 Exec command:
```
$ viking machine exec deathstar echo 1234
1234
```

#### 📺 Connect to the machine:
```
$ viking machine exec --tty deathstar /bin/bash
root@deathstar:~$
```

#### 🔑 Add SSH key from a file
```
$ viking key add --name starkey --passphrase dart ./id_rsa_star
Key starkey added.
```

#### 🆕 Generate SSH Key
```
$ viking key generate --name starkey2
Key starkey2 added.
```

#### 📋 Copy public SSH Key
```
$ viking key copy starkey2
Public key copied to your clipboard.
```

#### ⚙️ Custom config directory
Viking saves data locally. Set `VIKING_CONFIG_DIR` env variable for a custom directory. Use `viking config` to check the current config folder.

## 🤝  Missing a Feature?
Feel free to open a new issue, or contact me.

## 📘  License
Viking is provided under the [MIT License](https://github.com/d3witt/viking/blob/main/LICENSE).