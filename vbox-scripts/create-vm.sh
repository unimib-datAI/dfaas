#!/bin/sh

set -e

cd $(dirname "$0")

if [[ $# -ne 4 ]]; then
    echo "Comando non valido."
    echo "Utilizzo: $0 <nome-vm> <debian-iso-path> <natnetwork> <descrizione>"
    echo "  - nome-vm: nome della macchina virtuale"
    echo "  - debian-iso-path: percorso del file ISO di Debian"
    echo "  - natnetwork: nome della NAT Network di VirtualBox da utilizzare/creare"
    echo "  - descrizione: descrizione della VM"
    echo "Esempio: $0 MyVM \"/home/user/ISO/debian.iso\" MyNAT \"La mia macchina virtuale\""
    exit 1
fi

# Se si sta utilizzando Git Bash su Windows (se la directory "/c" esiste)
if [ -d "/c" ]; then
    # aggiungi la directory di VirtualBox al PATH, se non e' gia' presente
    command -v VBoxManage >/dev/null 2>&1 || \
        export PATH="$PATH:/c/Program Files/Oracle/VirtualBox"
fi

# Directory in cui sono collocati i file delle macchine virtuali di VirtualBox
VBOX_MACHINEFOLDER=$(VBoxManage list systemproperties | \
    grep -i "machine folder" | \
    sed 's/machine folder: \+/\$/g' | \
    cut -d'$' -f2)

VM_NAME=$1
DEBIAN_ISO_LOCAL_PATH="$2"
NATNETWORK_NAME=$3
VM_DESCRIPTION="$4"

VM_RAM_MB=2048 # 2 GB RAM
VM_CPUS=2
VM_VIDEO_MEMORY_MB=32
VM_HDD_SIZE_MB=122880 # 120 GB HDD

echo "Creazione macchina virtuale \"$VM_NAME\"..."
# Nota: l'opzione "--default" applica una configurazione hardware di default
# specifica per l'OS selezionato
VBoxManage createvm --name $VM_NAME --ostype Debian_64 --register --default

echo "Settings -> General -> Description: \"$VM_DESCRIPTION\""
VBoxManage modifyvm $VM_NAME --description "$VM_DESCRIPTION"

echo "Settings -> System -> Motherboard -> Base Memory: $VM_RAM_MB MB"
VBoxManage modifyvm $VM_NAME --memory $VM_RAM_MB

echo "Settings -> System -> Motherboard -> Boot Order: Optical, Hard Disk"
VBoxManage modifyvm $VM_NAME --boot1 dvd --boot2 disk --boot3 none --boot4 none

echo "Settings -> System -> Motherboard -> Pointing Device: PS/2 Mouse"
VBoxManage modifyvm $VM_NAME --mouse ps2

echo "Settings -> System -> Motherboard -> Hardware Clock in UTC Time: ON"
VBoxManage modifyvm $VM_NAME --rtcuseutc on

echo "Settings -> System -> Processor -> Processor(s): $VM_CPUS CPU(s)"
VBoxManage modifyvm $VM_NAME --cpus $VM_CPUS

echo "Settings -> Display -> Screen -> Video Memory: $VM_VIDEO_MEMORY_MB MB"
VBoxManage modifyvm $VM_NAME --vram $VM_VIDEO_MEMORY_MB

##### INIZIO CONFIGURAZIONE STORAGE #####

VM_HDD_FILE_PATH="$VBOX_MACHINEFOLDER/$VM_NAME/$VM_NAME.vdi"

echo "Creazione file VDI dell'hard-disk..."
echo "  - VDI file path: \"$VM_HDD_FILE_PATH\""
echo "  - Hard Disk size: $VM_HDD_SIZE_MB MB"
VBoxManage createmedium disk --filename "$VM_HDD_FILE_PATH" --size $VM_HDD_SIZE_MB

#echo "Rinomino i controller SATA e IDE esistenti..." # Non piu' necessario su VirtualBox 6.1
#VBoxManage storagectl $VM_NAME --name "DVD 1" --rename "IDE"
#VBoxManage storagectl $VM_NAME --name "HDD 1" --rename "SATA"

echo "Settings -> Storage -> Add optical drive (\"$DEBIAN_ISO_LOCAL_PATH\") to IDE controller"
# IDE device on channel Primary (--port 0) Master (--device 0)
VBoxManage storageattach $VM_NAME --storagectl "IDE" \
    --port 0 --device 0 --type dvddrive \
    --medium "$DEBIAN_ISO_LOCAL_PATH"

echo "Settings -> Storage -> Add hard disk (\"$VM_HDD_FILE_PATH\") to SATA controller"
# SATA port 0 (--port 0)
# Device should always be 0 (--device 0) if using SATA controller, because it only allows one device per port.
VBoxManage storageattach $VM_NAME --storagectl "SATA" \
    --port 0 --device 0 --type hdd \
    --medium "$VM_HDD_FILE_PATH"

##### FINE CONFIGURAZIONE STORAGE #####

echo "Settings -> Audio -> Enable Audio: OFF"
VBoxManage modifyvm $VM_NAME --audio none

echo "Settings -> Network -> Adapter 1 -> Attached to: NAT Network \"$NATNETWORK_NAME\""
VBoxManage modifyvm $VM_NAME --nic1 natnetwork --nat-network1 $NATNETWORK_NAME

echo "Settings -> USB -> Enable USB Controller: OFF"
VBoxManage modifyvm $VM_NAME --usb off

echo "Settings -> User Interface -> Show Mini ToolBar in Full-screen/Seamless: OFF"
VBoxManage setextradata $VM_NAME GUI/ShowMiniToolBar false

echo "La macchina virtuale \"$VM_NAME\" e' stata creata con successo."
