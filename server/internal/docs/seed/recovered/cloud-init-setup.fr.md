# Configuration cloud-init (administrateur)

Ce guide explique le fonctionnement de cloud-init dans PVMSS et comment
l'administrateur le prépare pour ses utilisateurs. Cloud-init configure une VM
à son premier démarrage sans s'y connecter : paquets, fichiers, commandes, etc.

## Fonctionnement

- Deux sources de **documents cloud-init** : les **templates**
  administrateur (`/admin/cloudinit-templates`, par cluster) et les
  **fichiers personnels** des utilisateurs (`/cloud-init`, 20 par
  utilisateur). Les deux sont des documents `#cloud-config` stockés dans la
  base PVMSS.
- À la création d'une VM, l'utilisateur choisit un document dans un
  sélecteur groupé. PVMSS écrit une **copie propre à la VM** nommée
  `pvmss-<vmid>.yml` dans le stockage de snippets du cluster, vérifie qu'elle
  est visible et l'attache à la VM en vendor data (`cicustom=vendor=…`). La
  copie fait foi : modifier le template ou le fichier ensuite ne change jamais
  les VM existantes.
- Le vendor data fusionne avec le user data que Proxmox génère depuis le
  formulaire (utilisateur, mot de passe, clés SSH, réseau). `packages`,
  `package_update`, `runcmd`, `bootcmd`, `write_files`, `apt`, `timezone`,
  `ntp`… s'appliquent tous. Une clé `users:` dans le document est écrasée par
  le compte généré - demandez aux utilisateurs de mettre comptes et clés dans
  le formulaire.
- Après création, l'onglet **Cloud-init** de la VM affiche le document.
  Quand **Autoriser le YAML cloud-init libre** est activé dans la politique,
  les utilisateurs peuvent le modifier : l'enregistrement écrase le fichier
  propre à la VM et s'applique au prochain démarrage.

## Prérequis : une cible d'écriture de snippets

L'API REST de Proxmox ne sait pas écrire de fichiers `snippets` ; PVMSS les
écrit via un répertoire monté dans son conteneur. Suivez **Activer les
documents cloud-init** dans le [guide administrateur](/docs/admin-guide) :
stockage partagé avec le type de contenu Snippets, montage de son répertoire
`snippets/` dans le conteneur, puis *Répertoire de snippets* et *Stockage de
snippets* sur le cluster dans **Admin › Clusters**.

Sans cible d'écriture, le sélecteur de document est masqué dans l'assistant et
une création portant un document est refusée avant qu'un VMID ne soit
consommé.

## Tâches de l'administrateur

1. Ouvrez **Admin > Templates cloud-init**.
2. Créez un template avec un libellé et le contenu `#cloud-config`.
3. Le portail valide l'en-tête `#cloud-config` et la syntaxe YAML ; il ne
   valide pas la sémantique cloud-init.
4. Activez le template pour qu'il apparaisse dans le sélecteur. Désactivez-le
   pour le masquer sans le supprimer.

Les templates sont statiques - pas de variables. Les valeurs propres à
l'utilisateur (utilisateur, mot de passe, clés SSH, réseau) viennent du
formulaire.

## Exemples de templates

Installation de paquets :

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - qemu-guest-agent
  - vim
  - htop
  - curl
runcmd:
  - systemctl enable --now qemu-guest-agent
```

Fichiers et commandes personnalisés :

```yaml
#cloud-config
timezone: Europe/Paris
write_files:
  - path: /etc/motd
    content: |
      Provisionné par PVMSS.
runcmd:
  - systemctl enable --now docker
```

## VM depuis image cloud et snippet de base

Les VM créées depuis une **image cloud** reçoivent en plus un snippet de base
fixe, `pvmss-baseline.yml`, s'il existe dans le même répertoire `snippets/` - 
pratique pour installer `qemu-guest-agent` sur tout le cluster. Son absence
est silencieuse, pas une erreur.

## Dépannage

- **Sélecteur masqué dans l'assistant** : le cluster n'a pas de cible
  d'écriture - vérifiez **Admin › Clusters** (badge « cloud-init : activé »).
- **Création refusée avec `cloudinit_write_unavailable`** : le répertoire
  n'est pas monté, pas inscriptible par l'utilisateur du conteneur (uid
  65532), ou l'identifiant de stockage ne correspond pas au stockage Proxmox
  qui le possède.
- **Document non appliqué** : sur un nœud, `qm config <vmid> | grep cicustom`
  doit afficher `vendor=<stockage>:snippets/pvmss-<vmid>.yml` ; consultez
  `/var/log/cloud-init.log` dans l'invité.
- **Changements sans effet** : la plupart des modules ne s'exécutent qu'au
  premier démarrage. Voir le [guide cloud-init](/docs/cloud-init-howto) pour
  la procédure `cloud-init clean`.
- **YAML invalide** : le portail rejette les documents dont le YAML est
  invalide ou sans en-tête `#cloud-config`.

## Limitations

- Templates statiques ; pas de variables.
- Seuls la syntaxe YAML et l'en-tête `#cloud-config` sont validés.
- Supprimer une VM dans PVMSS retire son fichier ; les VM supprimées
  directement dans Proxmox laissent des orphelins (comparer
  `snippets/pvmss-*.yml` avec `qm list`).
- Les documents sont stockés en clair ; ce n'est pas un endroit pour des
  secrets.
