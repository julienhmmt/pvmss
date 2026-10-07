# Configuration cloud-init (administrateur)

Les administrateurs PVMSS écrivent les modèles cloud-init ; les utilisateurs
en choisissent un à la création d'une VM. L'API REST de Proxmox ne peut pas
écrire de fichiers `snippets` (ses endpoints `upload` et `download-url`
n'acceptent que `iso`, `vztmpl` et `import`), donc **PVMSS n'écrit jamais
sur les nœuds**. Pour chaque modèle, il affiche une commande ; l'administrateur
la colle, en root, sur les nœuds qui doivent proposer le modèle. PVMSS lit
ensuite, via l'API, quels nœuds ont le fichier.

Pas de clé SSH, pas d'utilisateur dédié, pas d'assistant, pas de script de
préparation.

## Fonctionnement

```
Admin > Cloud-init : enregistrer un modèle
  │  PVMSS le fusionne par-dessus la baseline générée (qemu-guest-agent)
  │  nom du fichier = pvmss-tpl-<id du modèle>-<sha256[:12]>.yml   (adressé par contenu)
  ▼
La page affiche, par modèle : le fichier, « Sur n/m nœuds » et « Commande à coller »
  │  l'admin colle la commande, en root, sur chaque nœud qui doit le proposer
  │  (la commande écrit <stockage>:snippets/<fichier> via pvesm path)
  ▼
Vérifier (ou tout rechargement) : PVMSS liste <stockage>:snippets sur chaque nœud (API)

Création de VM
  │  le sélecteur ne propose que les modèles présents sur le nœud de la VM
  │  PVMSS revérifie que le fichier est sur ce nœud (API), sinon 409 cloudinit_not_published
  ▼
qm set <vmid> --cicustom vendor=<stockage>:snippets/pvmss-tpl-…yml
```

- Le modèle est de la **vendor data** : il se fusionne avec la user data que
  Proxmox génère depuis le formulaire (utilisateur, mot de passe, clés SSH,
  réseau). Comptes et clés vont dans le formulaire ; une clé `users:` dans un
  modèle est écrasée.
- **Vous choisissez les nœuds.** Un modèle n'est proposé que sur les nœuds
  qui ont son fichier. Collez-le sur un nœud aujourd'hui, sur les autres plus
  tard.
- Modifier un modèle donne un **nouveau** nom de fichier, donc une nouvelle
  commande à coller. Les VM existantes gardent le fichier avec lequel elles ont
  été créées : ne supprimez pas un ancien fichier qu'une VM utilise encore.
- La baseline (qemu-guest-agent) est fusionnée sous chaque modèle et existe
  aussi seule sous le nom `pvmss-baseline-<hash>.yml` (Admin > Baseline
  cloud-init), pour les VM cloud-image créées sans modèle. Sans ce fichier sur
  le nœud, ces VM démarrent avec les seules clés natives (pas d'installation
  de l'agent invité).

## Mise en place, pas à pas

### 1. Activer le type de contenu Snippets sur le stockage (une fois, un nœud)

La configuration des stockages vaut pour tout le cluster. En root sur un
nœud :

```sh
STORAGE=local
CUR=$(pvesh get /storage/$STORAGE --output-format json | perl -MJSON -0ne 'print decode_json($_)->{content}')
case ",$CUR," in
  *,snippets,*) echo "snippets déjà activé sur $STORAGE" ;;
  *) pvesm set "$STORAGE" --content "$CUR,snippets" && echo "snippets activé sur $STORAGE" ;;
esac
```

(Ou dans l'interface : Centre de données > Stockage > `local` > Éditer >
Contenu : ajouter Snippets.) `local` est le plus simple : quelques Ko par
modèle, une copie sur chaque nœud qui en a besoin. Un stockage partagé
(`nfs`, `cifs`, `cephfs`) fonctionne aussi : un seul collage sert alors tous
les nœuds. Le stockage objet S3 n'est pas pris en charge.

### 2. Choisir le stockage dans PVMSS

**Infrastructure > Clusters > Modifier > Modèles cloud-init** : choisissez
le stockage de snippets, enregistrez. Le cluster affiche « cloud-init :
activé ». Un stockage vide désactive les modèles cloud-init sur ce cluster.

### 3. Écrire un modèle

**Admin > Cloud-init > Nouveau modèle**, contenu `#cloud-config`,
enregistrez.

### 4. Coller la commande sur les nœuds choisis

Dans la ligne du modèle, ouvrez **Commande à coller**, cliquez sur
**Copier**, et collez-la dans un shell root sur chaque nœud qui doit proposer
le modèle (le shell web de Proxmox convient). Elle ressemble à :

```sh
F=$(pvesm path local:snippets/pvmss-tpl-web-3f2a9c81d0e4.yml) && mkdir -p "${F%/*}" && cat > "$F" <<'PVMSS_EOF'
#cloud-config
...
PVMSS_EOF
```

Le here-document entre guillemets écrit le contenu tel quel (aucune
expansion de `$`). Faites de même pour la baseline si vous créez des VM
cloud-image sans modèle (**Admin > Baseline cloud-init**).

### 5. Vérifier

Cliquez sur **Vérifier** dans la page des modèles. La ligne affiche « Sur
n/m nœuds » et indique chaque nœud présent ou absent. Créez ensuite une VM
avec le modèle sur l'un de ces nœuds, puis sur le nœud :

```sh
qm config <vmid> | grep cicustom
```

## Exploitation courante

| Situation | Action |
| --- | --- |
| Proposer un modèle sur un autre nœud | Coller sa commande sur ce nœud, Vérifier |
| Nœud ajouté ou réinstallé | Coller les commandes des modèles qu'il doit proposer (et la baseline) |
| Modèle modifié | Nouveau nom de fichier : coller la nouvelle commande sur les nœuds, Vérifier |
| Mise à jour de PVMSS qui change la baseline | Tous les noms de fichiers changent : coller les nouvelles commandes |
| Supprimer une ancienne version | `rm` sur le nœud, seulement si aucune VM ne l'utilise (`grep -l <fichier> /etc/pve/qemu-server/*.conf`) |
| VM supprimée | Rien à faire. Un ancien fichier par VM (`pvmss-<vmid>.yml`, anciennes versions) est journalisé et laissé sur le nœud : `rm` à la main |

## Retirer l'ancienne publication SSH

Les anciennes versions de PVMSS publiaient par SSH. Après la mise à jour :

- Retirez `PVMSS_SSH_KEY_FILE` et le montage de la clé de Compose / des
  valeurs Helm (PVMSS journalise un avertissement tant qu'elle est définie)
  et supprimez le fichier de clé.
- Sur chaque nœud, en root :

  ```sh
  userdel -r pvmss 2>/dev/null; userdel -r pvmss-snippets 2>/dev/null
  rm -f /usr/local/bin/pvmss-snippet /etc/pvmss-snippet.conf
  ```

  Retirez ensuite tout bloc `Match User pvmss…` ajouté à
  `/etc/ssh/sshd_config`, vérifiez avec `sshd -t`, puis
  `systemctl reload ssh`.
- Les fichiers déjà présents sur les nœuds continuent de fonctionner : les
  VM les désignent par leur nom.

## Dépannage

| Symptôme | Cause / correction |
| --- | --- |
| « cloud-init : désactivé » sur le cluster | Aucun stockage de snippets choisi (étape 2) |
| Liste des stockages vide dans le formulaire du cluster | Type de contenu Snippets non activé (étape 1) |
| Collé, toujours « absent » | Collé sur un autre nœud, ou dans un autre stockage que celui de l'étape 2 : `pvesm list <stockage> --content snippets` sur le nœud |
| `pvesm: storage '…' does not exist` | Faute de frappe dans l'id, ou stockage indisponible sur ce nœud |
| Le nœud affiche « node is offline » | Le nœud est arrêté : rien ne peut y être lu |
| Modèle absent de l'assistant de création | Son fichier n'est pas sur le nœud de la VM : collez-le là |
| Création refusée avec `cloudinit_not_published` | Idem : le fichier n'est pas sur le nœud choisi |
| La VM démarre sans l'effet du modèle | `qm config <vmid> \| grep cicustom`, puis `cloud-init status --long` dans l'invité |
