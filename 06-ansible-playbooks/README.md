# vm-dfaasctrl ansible

Prerequisiti sul client:

- ansible
- `sudo apt update && sudo apt install rsync`
- aver aggiunto la chiave ssh del server a `known_hosts`

Eseguire i file nell'ordine:

- `script-setup-client.sh`
- `script-setup-server.sh`
- `script-deploy-functions.sh`
- `script-deploy.sh`

**N.B.** In alternativa, mentre hai aperto il file di un playbook, puoi premere `CTRL+ALT+T` ed eseguire la **task** in *VSCode*, grazie al file `.vscode/tasks.json`
