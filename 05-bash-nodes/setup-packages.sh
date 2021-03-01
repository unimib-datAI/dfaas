#!/bin/bash

set -ex

# Assicurati di essere root
if [[ $EUID -ne 0 ]]; then
    echo "This script must be run as root" 1>&2
    exit 1
fi

# Ottieni il nome dell'unico utente (sudoer) presente nel sistema
SUDOER_USERNAME=$(ls /home)

# Posizionati nella home directory dell'utente sudoer
cd /home/$SUDOER_USERNAME

# Aggiorna elenchi pacchetti APT
apt-get update

# Installazione pacchetti fondamentali
apt-get install -y git htop nano tmux tree wget zip

##### INSTALLAZIONE DOCKER #####

# Installa pacchetti necessari per Docker
apt-get install -y apt-transport-https ca-certificates curl \
    gnupg2 software-properties-common

# Aggiungi la chiave GPG ufficiale di Docker
curl -sSL https://download.docker.com/linux/debian/gpg | apt-key add -
# Verifica che la chiave sia stata aggiunta correttamente
apt-key fingerprint 9DC858229FC7DD38854AE2D88D81803C0EBFCD88

# Aggiungi il repository APT *stable* di Docker
add-apt-repository \
    "deb [arch=amd64] https://download.docker.com/linux/debian \
    $(lsb_release -cs) \
    stable"
# Riaggiorna elenchi pacchetti dopo aver aggiunto il repository
apt-get update

# Installa Docker
apt-get install -y docker-ce docker-ce-cli containerd.io

# Verifica che Docker sia stato installato correttamente
# (tale comando fallisce se l'applicazione non e' stata installata)
docker --version

# Aggiungi l'utente sudoer al gruppo "docker"
usermod -aG docker $SUDOER_USERNAME

##### INSTALLAZIONE DOCKER-COMPOSE #####

# Installa il file eseguibile di Docker-Compose
curl -L "https://github.com/docker/compose/releases/download/1.25.4/docker-compose-$(uname -s)-$(uname -m)" \
    -o /usr/local/bin/docker-compose
# Imposta i permessi di esecuzione sul file
chmod +x /usr/local/bin/docker-compose

# Installa la Bash completion per Docker-Compose
curl -L "https://raw.githubusercontent.com/docker/compose/1.25.4/contrib/completion/bash/docker-compose" \
    -o /etc/bash_completion.d/docker-compose

# Verifica che Docker-Compose sia stato installato correttamente
# (tale comando fallisce se l'applicazione non e' stata installata)
docker-compose --version

##### INSTALLAZIONE OPENFAAS #####

# Activate the Docker Swarm mode
docker swarm init

# Clona in locale il repository Git di OpenFaaS
git clone https://github.com/openfaas/faas # Non si puo usare --depth 1 perche' serve il commit specifico
cd faas
git checkout d05d0a76a5b7274df40426f5f22282a6d9245257

# Imposta password per l'utente "admin" di openfaas
sed -i 's/^\(secret=\).*$/\1faaspass2020/' deploy_stack.sh

# Avvia lo stack!
./deploy_stack.sh

# Nota: con il comando "docker stack services func" puoi vedere le porte esposte, che sono:
#   - openfaas gateway: 8080
#   - prometheus: 9090
