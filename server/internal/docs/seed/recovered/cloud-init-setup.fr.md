# Configuration cloud-init (administrateur)

Ce guide explique le fonctionnement de cloud-init dans PVMSS et comment les
administrateurs le préparent pour leurs utilisateurs. Cloud-init configure une
VM au premier démarrage sans connexion : paquets, fichiers, commandes, etc.

## Fonctionnement

- **Seuls les administrateurs écrivent les documents cloud-init** : les
  **templates** de **Admin > Modèles cloud-init** (par cluster,
  `#cloud-config`). Les utilisateurs en choisissent un à la création d'une
  VM, ou en changent plus tard depuis l'onglet **Cloud-init** de la VM. Les
  utilisateurs n'écrivent jamais de YAML.
- Enregistrer un template le **publie** : PVMSS le fusionne par-dessus la
  base PVMSS (qemu-guest-agent) et écrit le résultat, par SSH, sous forme
  d'un fichier immuable `pvmss-tpl-<id>-<hash>.yml` dans le stockage de
  snippets de **chaque nœud**, puis vérifie via l'API Proxmox que chaque
  nœud le liste. La page affiche le résultat par nœud (« 3/3 nœuds »).
- La création d'une VM n'écrit jamais de fichier : PVMSS vérifie que le
  fichier du template est présent sur le nœud de la VM, puis y fait pointer
  la VM (`cicustom=vendor=…`). Un template absent du nœud est refusé avant
  la création de la VM.
- Modifier un template publie un **nouveau** fichier : les VM gardent la
  version avec laquelle elles ont été créées.
- Les vendor data fusionnent avec les user data générées par Proxmox à partir
  du formulaire de la VM (utilisateur, mot de passe, clés SSH, réseau).
  `packages`, `package_update`, `runcmd`, `bootcmd`, `write_files`, `apt`,
  `timezone`, `ntp`… s'appliquent. Une clé `users:` dans le document est
  écrasée par le compte généré : les comptes et les clés se renseignent dans
  le formulaire.

## Prérequis : publication SSH

L'API REST de Proxmox ne sait pas écrire de fichiers `snippets` ; PVMSS les
publie donc par SSH, via un petit utilitaire (`pvmss-snippet`) installé sur
chaque nœud pour un utilisateur dédié `pvmss`. PVMSS a besoin :

- de la clé privée globale, `PVMSS_SSH_KEY_FILE` (paramètre du serveur) ;
- par cluster, dans **Infrastructure > Clusters > Modifier** : du stockage de
  snippets, de l'utilisateur SSH, du port SSH et des clés d'hôte épinglées ;
- d'un accès réseau de PVMSS vers **l'IP de chaque nœud telle que listée
  dans `/cluster/status`**, sur le port SSH (elle peut différer de l'URL de
  l'API).

Les commandes ci-dessous sont prêtes à l'emploi : renseignez d'abord les
variables. La référence complète (Compose, Helm, Kubernetes, rotation de
clé, désinstallation) est `docs/cloud-init-ssh.md` dans le dépôt PVMSS.

**1. Clé de PVMSS** (poste de travail) :

```sh
ssh-keygen -t ed25519 -N '' -C pvmss -f pvmss_ed25519
```

ed25519 est recommandé ; ECDSA et RSA fonctionnent aussi (RSA : 2048 bits
minimum, 3072+ recommandé, par exemple `ssh-keygen -t rsa -b 4096 …`). Sans
passphrase.

Fournissez la clé privée à PVMSS : montée en lecture seule, lisible par
l'uid 65532 (utilisateur du conteneur), avec
`PVMSS_SSH_KEY_FILE=/etc/pvmss/ssh/id_ed25519`.

- Compose : `- ./pvmss_ed25519:/etc/pvmss/ssh/id_ed25519:ro`, puis
  `sudo chown 65532:65532 pvmss_ed25519 && sudo chmod 0400 pvmss_ed25519`.
- Helm : `kubectl -n pvmss create secret generic pvmss-ssh
  --from-file=id_ed25519=./pvmss_ed25519` et
  `--set cloudInit.sshKeySecret=pvmss-ssh`.

Après redémarrage, la clé publique apparaît dans **Infrastructure > Clusters >
Modifier**.

**2. Type de contenu Snippets** (un seul nœud, en root, une fois - la
configuration des stockages est commune au cluster ; ou Datacenter >
Storage > Edit > Content) :

```sh
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in *,snippets,*) echo deja ;; *) pvesm set "$STORAGE" --content "$CUR,snippets" ;; esac
```

IP des nœuds auxquelles PVMSS se connectera :

```sh
pvesh get /cluster/status --output-format json \
  | perl -MJSON -0ne 'print "$_->{name} $_->{ip}\n" for grep { $_->{type} eq "node" } @{decode_json($_)}'
```

**3. Chaque nœud.** PVMSS sert son script de préparation sur
`/api/v1/pvmss-node-setup.sh` (embarqué dans le binaire ; c'est le même
fichier que `tools/pvmss-node-setup.sh` dans le dépôt). La commande exacte,
avec l'URL de ce PVMSS, le stockage, l'utilisateur et la clé, est affichée
avec un bouton de copie dans **Infrastructure > Clusters > Modifier**, à côté
d'un lien **Voir le script**. En root sur chaque nœud :

```sh
PVMSS=https://pvmss.example.com       # URL de PVMSS vue depuis le nœud
curl -fsSL "$PVMSS/api/v1/pvmss-node-setup.sh" \
  | sh -s -- --storage local --user pvmss --key 'ssh-ed25519 AAAA... pvmss'
```

Certificat PVMSS auto-signé : `curl -kfsSL`. Pour tous les nœuds d'un coup,
depuis un poste ayant l'accès SSH root :

```sh
NODES="192.168.1.11 192.168.1.12 192.168.1.13"
STORAGE=local
PUBKEY=$(cat pvmss_ed25519.pub)
for n in $NODES; do
  ssh root@"$n" "curl -fsSL '$PVMSS/api/v1/pvmss-node-setup.sh' | sh -s -- --storage $STORAGE --user pvmss --key '$PUBKEY'"
done
```

Nœuds sans accès à PVMSS : copiez-y `tools/pvmss-node-setup.sh` (`scp`) et
lancez `sh pvmss-node-setup.sh` avec les mêmes options. Le script est
idempotent ; un téléchargement tronqué n'exécute rien.

L'URL de la commande est l'adresse de la page (HTTP ou HTTPS, IP ou FQDN,
n'importe quel port) : les nœuds doivent pouvoir la joindre. Le formulaire
prévient quand c'est `localhost` ou le port 5173 de Vite ; derrière un
reverse proxy avec sous-chemin, ajoutez le sous-chemin à la main. Le script
refuse une clé qui n'est pas une seule ligne valide (RSA de moins de 2048 bits
compris), `root` comme utilisateur et un stockage sans Snippets ; sur NFS/CIFS
sans ACL, il se rabat sur le groupe du répertoire, et s'arrête avec un
message explicite si l'utilisateur ne peut toujours pas y écrire.

Le script installe `/usr/local/bin/pvmss-snippet`, écrit
`/etc/pvmss-snippet.conf`, crée l'utilisateur `pvmss` avec un accès en
écriture au seul répertoire `snippets/` du stockage, installe la clé avec une
commande forcée (pas de shell, pas de redirection) et affiche la clé d'hôte
du nœud.

**4. Vérifier un nœud** (poste de travail) :

```sh
NODE=192.168.1.11
SSH="ssh -i pvmss_ed25519 -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new pvmss@$NODE"
$SSH check                                               # pvmss-snippet ok <répertoire snippets>
printf '#cloud-config\n' | $SSH write pvmss-selftest.yml
ssh root@$NODE "pvesm list $STORAGE --content snippets | grep pvmss-selftest"
$SSH remove pvmss-selftest.yml
$SSH id                                                  # doit être refusé (usage: ...)
```

**5. Infrastructure > Clusters > Modifier.** L'utilisateur SSH exige des clés d'hôte
épinglées, et **Scanner les clés d'hôte** exige un cluster enregistré. Au
choix :

- coller les clés d'hôte et enregistrer une seule fois :

  ```sh
  for n in $NODES; do ssh-keyscan -t ed25519 "$n" 2>/dev/null; done
  ```

  puis renseigner le stockage de snippets, l'utilisateur SSH `pvmss`, le
  port, coller les lignes dans **Clés d'hôte épinglées**, **Enregistrer** ;
- ou renseigner le stockage de snippets et le port, **Enregistrer** ;
  rouvrir, **Scanner les clés d'hôte**, comparer avec les lignes affichées
  par le script, renseigner l'utilisateur SSH `pvmss`, **Enregistrer**.

Les clés d'hôte sont toujours vérifiées. Le badge passe à « cloud-init :
activé » et PVMSS republie la base et les templates en arrière-plan. S'il
reste désactivé, le badge indique l'élément manquant : pas de clé SSH
(`PVMSS_SSH_KEY_FILE`), pas d'utilisateur SSH, pas de clé d'hôte épinglée ou
pas de stockage de snippets.

PVMSS n'envoie jamais que `pvmss-snippet write <nom>` (contenu sur l'entrée
standard) et `pvmss-snippet remove <nom>` ; l'utilitaire vérifie le nom
(`pvmss-*.yml`) et maîtrise le répertoire. PVMSS n'envoie jamais de chemin
ni de commande shell, et la clé ne peut pas ouvrir de shell.

Sans publication SSH, le choix du template est masqué dans l'assistant et
une demande de création portant un template est refusée avant qu'un VMID
soit consommé.

## Tâches de l'administrateur

1. Ouvrez **Admin > Modèles cloud-init**.
2. Créez un template avec un libellé et le contenu `#cloud-config`.
   L'enregistrement le publie ; vérifiez la colonne « Publié ».
3. Désactivez un template pour le masquer aux utilisateurs sans le
   supprimer. Supprimer un template ne casse jamais les VM qui l'utilisent :
   leur fichier reste sur les nœuds.
4. Après l'ajout ou la réinstallation d'un nœud (ou si un nœud était hors
   ligne lors d'un enregistrement), cliquez sur **Publier sur tous les
   nœuds**.

Les templates sont statiques : il n'y a pas de variables de template. Les
valeurs propres à l'utilisateur (utilisateur, mot de passe, clés SSH, réseau)
viennent du formulaire de la VM.

## Exemples de templates

Installation de paquets :

```yaml
#cloud-config
package_update: true
packages:
  - vim
  - htop
  - curl
```

Fichiers et commandes personnalisés :

```yaml
#cloud-config
timezone: Europe/Paris
write_files:
  - path: /etc/motd
    content: |
      Provisionnée par PVMSS.
runcmd:
  - systemctl enable --now docker
```

## La base

La base générée (voir **Admin > Baseline cloud-init**) installe et active
`qemu-guest-agent`. Elle est fusionnée sous chaque template, et publiée seule
(`pvmss-baseline-<hash>.yml`) pour les VM cloud-image créées sans template.
Si elle n'est pas présente sur le nœud de la VM, la VM démarre quand même
avec ses clés natives et sa page de détail indique que la base n'a pas été
livrée.

## Dépannage

- **« n/m nœuds » dans la colonne Publié** : l'erreur du nœud en échec est
  affichée en dessous.
  - `host ... is not in the cluster's pinned host keys` / `host key
    mismatch` : scannez à nouveau les clés d'hôte et comparez les empreintes
    (une différence sans réinstallation est un signal d'alerte).
  - `ssh handshake ... unable to authenticate` : la clé PVMSS n'est pas dans
    `~pvmss/.ssh/authorized_keys` sur ce nœud ; relancez le script
    d'installation.
  - `written, but Proxmox does not list ...` : le répertoire de l'utilitaire
    (`/etc/pvmss-snippet.conf`) n'est pas le répertoire `snippets/` du
    stockage, ou le stockage n'a pas le type de contenu Snippets sur ce nœud.
  - `node is offline` : publiez à nouveau quand il est revenu.
- **Création refusée avec `cloudinit_not_published`** : le template n'est
  pas sur le nœud de la VM ; publiez à nouveau.
- **Document non appliqué** : sur un nœud, `qm config <vmid> | grep
  cicustom` doit afficher `vendor=<stockage>:snippets/pvmss-...yml` ;
  consultez `/var/log/cloud-init.log` dans l'invité.
- **Modifications sans effet** : la plupart des modules ne s'exécutent qu'au
  premier démarrage. Voir le [guide cloud-init](/docs/cloud-init-howto) pour
  la procédure `cloud-init clean`.
- **YAML invalide** : le portail rejette les documents qui ne sont pas du
  YAML valide ou sans l'en-tête `#cloud-config`.

## Limites

- Templates statiques ; pas de variables de template.
- Seuls la syntaxe YAML et l'en-tête `#cloud-config` sont validés.
- Les anciennes versions publiées ne sont pas supprimées des nœuds (quelques
  Ko chacune).
- Les documents sont stockés en clair sur les nœuds ; ils ne doivent pas
  contenir de secrets.
