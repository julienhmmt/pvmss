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
publie donc par SSH, via un petit utilitaire installé sur chaque nœud.

1. **Stockage** : dans Proxmox, ajoutez **Snippets** aux types de contenu
   d'un stockage disponible sur chaque nœud (Datacenter > Storage > Edit).
   `local` convient : le fichier est écrit sur chaque nœud.
2. **Clé PVMSS** : générez une paire de clés (`ssh-keygen -t ed25519 -N ''
   -f pvmss_ed25519`) et fournissez la clé privée à PVMSS avec
   `PVMSS_SSH_KEY_FILE` (fichier en lecture seule ; avec Helm, un Secret
   nommé dans `cloudInit.sshKeySecret`). La clé publique est affichée dans
   **Admin > Clusters > Modifier**.
3. **Chaque nœud**, en root : `sh pvmss-node-setup.sh --storage <stockage>
   --key '<clé publique PVMSS>'` (la commande exacte est affichée dans le
   formulaire du cluster). Le script installe `/usr/local/bin/pvmss-snippet`,
   crée l'utilisateur dédié `pvmss` avec un accès en écriture au seul
   répertoire `snippets/` du stockage, installe la clé avec une commande
   forcée (pas de shell, pas de redirection) et affiche la clé d'hôte du
   nœud.
4. **Admin > Clusters > Modifier** : renseignez le stockage de snippets,
   l'utilisateur SSH (`pvmss`) et le port, cliquez sur **Scanner les clés
   d'hôte**, comparez les empreintes avec celles affichées par le script,
   puis enregistrez. Les clés d'hôte sont toujours vérifiées. PVMSS republie
   la base et les templates en arrière-plan ; le badge du cluster passe à
   « cloud-init : activé ».

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
