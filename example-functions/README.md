# example-functions

Funzioni di esempio per *OpenFaaS*.

---

Casomai servisse, per creare le funzioni ho seguito i seguenti passaggi:

## 1 Preliminari

Installare il client di *OpenFaaS* `faas-cli` (es. tramite *Chocolatey* su *Windows 10*):

```bash
sudo choco install -y faas-cli
```

Effettuare il login nei server:

```bash
echo faaspass2020 | faas-cli login --gateway node01.dfaas.lvh.me:18801 --password-stdin
echo faaspass2020 | faas-cli login --gateway node02.dfaas.lvh.me:18802 --password-stdin
echo faaspass2020 | faas-cli login --gateway node03.dfaas.lvh.me:18803 --password-stdin
```

## 2 Creazione funzioni

Restando posizionato in questa stessa directory:

```bash
faas-cli template store pull golang-http
```

Verrà creata la cartella `template`.

```bash
faas-cli new --lang golang-http funca
faas-cli new --lang golang-http funcb
faas-cli new --lang golang-http funcc
```

Verranno create le cartelle `func*`, i file `func*.yml` e il file `.gitignore`.

## 3 Deploy funzioni

Vedi playbook *Ansible* `script-deploy-functions.sh`.
