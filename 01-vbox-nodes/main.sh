#!/bin/sh

set -e

cd $(dirname "$0")

# Se si sta utilizzando Git Bash su Windows (se la directory "/c" esiste)
if [ -d "/c" ]; then
    # aggiungi la directory di VirtualBox al PATH, se non e' gia' presente
    command -v VBoxManage >/dev/null 2>&1 || \
        export PATH="$PATH:/c/Program Files/Oracle/VirtualBox"
fi

##### INIZIO CONFIGURAZIONE #####

# URL per il download della ISO di Debian
DEBIAN_ISO_DOWNLOAD_URL="https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-10.9.0-amd64-netinst.iso"
# Percorso locale del file ISO di Debian
DEBIAN_ISO_LOCAL_PATH="C:\\Users\\Motte\\DatiMotteBig\\ISO\\debian-10.9.0-amd64-netinst.iso"

# Nome della rete di tipo NAT Network da creare e utilizzare
NATNETWORK_NAME="DFaaSNAT"
# Subnet ID di classe C per la NAT Network
# Esempio: scrivi 42 per ottenere la rete 192.168.42.0/24
NATNETWORK_SUBNET_ID=15

# Prefisso per i nomi delle VM
# Esempio: "Pippo" -> "Pippo01"
VM_NAME_PREFIX="DFaaSNode"

# Numero di macchine virtuali da creare
VMS_COUNT=3

##### FINE CONFIGURAZIONE #####

# Se l'immagine ISO di Debian non e' presente in locale, effettua il download
if [ ! -f "$DEBIAN_ISO_LOCAL_PATH" ]; then
    echo "Download di Debian..."
    echo "  - Remote ISO download URL: $DEBIAN_ISO_DOWNLOAD_URL"
    echo "  - ISO file local path: $DEBIAN_ISO_LOCAL_PATH"
    wget $DEBIAN_ISO_DOWNLOAD_URL -O "$DEBIAN_ISO_LOCAL_PATH"
    echo
fi

echo "Creazione NAT Network \"$NATNETWORK_NAME\"..."
VBoxManage natnetwork add --netname $NATNETWORK_NAME \
    --network "192.168.$NATNETWORK_SUBNET_ID.0/24" \
    --enable \
    --dhcp on || \
        echo "La NAT Network esiste gia'."
echo

for i in $(seq 1 $VMS_COUNT); do
    zi=$(printf "%02d" $i)
    VM_NAME=$VM_NAME_PREFIX$zi
    VM_IP_NUM=1$zi
    VM_DESC="Macchina virtuale $VM_NAME. SSH: 192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM:122$zi"

    ../vbox-scripts/create-vm.sh $VM_NAME "$DEBIAN_ISO_LOCAL_PATH" $NATNETWORK_NAME "$VM_DESC"
    echo

    echo "Creazione regole port-forwarding..."

    echo "  - [SSH$zi] localhost:122$zi -> 192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM:22"
    VBoxManage natnetwork modify --netname DFaaSNAT --port-forward-4 \
        "SSH$zi:tcp:[127.0.0.1]:122$zi:[192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM]:22" || \
            echo "La regola di port-forwarding esiste gia'."

    echo "  - [OPENFAAS$zi] localhost:188$zi -> 192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM:8080"
    VBoxManage natnetwork modify --netname DFaaSNAT --port-forward-4 \
        "OPENFAAS$zi:tcp:[127.0.0.1]:188$zi:[192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM]:8080" || \
            echo "La regola di port-forwarding esiste gia'."

    echo "  - [PROMETHEUS$zi] localhost:199$zi -> 192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM:9090"
    VBoxManage natnetwork modify --netname DFaaSNAT --port-forward-4 \
        "PROMETHEUS$zi:tcp:[127.0.0.1]:199$zi:[192.168.$NATNETWORK_SUBNET_ID.$VM_IP_NUM]:9090" || \
            echo "La regola di port-forwarding esiste gia'."

    echo
done

echo "Tutte le macchine virtuali sono state create con successo."
