# scripts/

Raccolta di script per il provisioning automatico di nodi DFaaS su VM Multipass.

## Struttura

```
scripts/
├── provision.sh                          # Entry point: crea la VM e lancia il playbook
├── ansible/
│   ├── setup_playbook.yaml               # Playbook Ansible: installa l'intero stack DFaaS
│   └── charts/
│       └── values-agent-openwhisk.yaml   # Helm values overlay per l'agent con OpenWhisk
└── lib/
    └── e2e-k3s-common.sh                 # Libreria bash per la gestione VM (Multipass + k3s)
```

---

## Flusso di provisioning

```
provision.sh
  │
  ├─ e2e_ensure_vm_running      # crea/avvia la VM Multipass
  ├─ e2e_sync_project_to_vm     # copia il repo su /home/ubuntu/dfaas nella VM
  ├─ genera inventory Ansible   # IP + SSH key della VM
  │
  └─ ansible-playbook setup_playbook.yaml
       │
       ├─ Installa K3S
       ├─ Builda immagini agent e forecaster
       ├─ Installa Helm + HAProxy + Prometheus
       ├─ Installa OpenFaaS  ──┐  in base a
       │  oppure OpenWhisk    ─┘  faas_platform
       ├─ Installa DFaaS agent (con API key OpenWhisk automatica)
       └─ Installa DFaaS Forecaster
```

---

## Quickstart

### Prerequisiti

- [Multipass](https://multipass.run/) installato sull'host
- [Ansible](https://docs.ansible.com/) installato sull'host (`pip install ansible`)

### Provisioning con OpenFaaS (default)

```bash
./scripts/provision.sh
```

### Provisioning con OpenWhisk

```bash
./scripts/provision.sh --faas-platform openwhisk
```

### VM personalizzata

```bash
./scripts/provision.sh \
  --vm-name dfaas-node-1 \
  --cpus 4 \
  --memory 8G \
  --disk 30G \
  --faas-platform openwhisk
```

### Con valori agent personalizzati (bootstrap nodes, node type, ecc.)

```bash
./scripts/provision.sh \
  --faas-platform openwhisk \
  -e "agent_values_file=/home/ubuntu/my-node-values.yaml"
```

> Il file `agent_values_file` deve essere presente **sulla VM** prima dell'esecuzione.
> Copiarlo con: `scp my-node-values.yaml ubuntu@<vm-ip>:/home/ubuntu/`

---

## Riferimento opzioni `provision.sh`

| Opzione | Default | Env var | Descrizione |
|---------|---------|---------|-------------|
| `--vm-name NAME` | `dfaas-node` | `VM_NAME` | Nome della VM Multipass |
| `--faas-platform PLAT` | `openfaas` | `FAAS_PLATFORM` | Piattaforma FaaS: `openfaas` o `openwhisk` |
| `--cpus N` | `4` | `VM_CPUS` | Numero di vCPU |
| `--memory SIZE` | `8G` | `VM_MEMORY` | Memoria RAM |
| `--disk SIZE` | `30G` | `VM_DISK` | Dimensione disco |
| `--skip-sync` | — | `SKIP_SYNC` | Salta la sincronizzazione del progetto sulla VM |
| `-h, --help` | — | — | Mostra l'help |

---

## Riferimento variabili `setup_playbook.yaml`

| Variabile | Default | Descrizione |
|-----------|---------|-------------|
| `faas_platform` | `openfaas` | Piattaforma FaaS da installare (`openfaas` o `openwhisk`) |
| `project_dir` | `{{ playbook_dir }}` | Path del repo DFaaS sull'host remoto |
| `agent_values_file` | `""` | Path opzionale a un file di valori Helm per l'agent |
| `openfaas_url` | `http://127.0.0.1:31112` | URL interno dell'OpenFaaS gateway |

Per eseguire il playbook direttamente senza `provision.sh`:

```bash
ansible-playbook scripts/ansible/setup_playbook.yaml \
  -i my-inventory.ini \
  -e "faas_platform=openwhisk" \
  -e "project_dir=/home/ubuntu/dfaas"
```

---

## `lib/e2e-k3s-common.sh`

Libreria bash condivisa per la gestione di VM Multipass e cluster k3s. Sourceable da qualsiasi script:

```bash
source scripts/lib/e2e-k3s-common.sh
e2e_set_log_prefix "mio-script"
```

Funzioni principali:

| Funzione | Descrizione |
|----------|-------------|
| `e2e_ensure_vm_running NAME [cpus] [mem] [disk]` | Crea o avvia una VM Multipass |
| `e2e_cleanup_vm` | Elimina la VM (`KEEP_VM=true` per preservarla) |
| `e2e_get_vm_ip` | Restituisce l'IP della VM (`VM_NAME`) |
| `e2e_auto_detect_vm [default]` | Rileva automaticamente la VM in esecuzione |
| `e2e_ssh_exec IP CMD` | Esegue un comando sulla VM via SSH (con retry) |
| `e2e_scp_to_vm SRC IP DEST` | Copia un file sulla VM |
| `e2e_scp_from_vm IP SRC DEST` | Scarica un file dalla VM |
| `e2e_vm_exec CMD` | Esegue un comando sulla VM con KUBECONFIG preimpostato |
| `e2e_sync_project_to_vm ROOT NAME DIR` | Sincronizza il repo sulla VM via tar+scp |
| `e2e_install_vm_dependencies` | Installa Docker, JDK21, Helm opzionale |
| `e2e_install_k3s` | Installa k3s sulla VM |
| `e2e_setup_local_registry` | Avvia un registry Docker locale e lo configura in k3s |
| `e2e_push_images_to_registry IMG...` | Pusha immagini Docker nel registry |
| `e2e_import_images_to_k3s IMG...` | Importa immagini Docker direttamente in k3s |
| `e2e_wait_for_deployment NS NAME [timeout]` | Attende che un Deployment k8s sia pronto |

Variabili d'ambiente riconosciute dalla libreria:

| Variabile | Default | Descrizione |
|-----------|---------|-------------|
| `VM_NAME` | — | Nome della VM target |
| `VM_IP` | — | IP esplicito (bypass autodetect) |
| `E2E_VM_USER` | `ubuntu` | Utente SSH della VM |
| `E2E_SSH_KEY` | autodetect | Path della chiave SSH privata |
| `KEEP_VM` | `false` | Se `true`, non elimina la VM al cleanup |
| `MULTIPASS_PURGE` | `auto` | Policy purge Multipass (`always/never/auto`) |
| `K3S_VERSION` | `v1.32.2+k3s1` | Versione k3s da installare |
| `HELM_VERSION` | `3.16.4` | Versione Helm da installare |
