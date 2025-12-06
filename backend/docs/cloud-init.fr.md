# Support Cloud-Init dans PVMSS

## Vue d'ensemble

PVMSS fournit un support cloud-init pour les VMs Proxmox, permettant aux utilisateurs d'appliquer des configurations cloud-init avancées lors de la création de VM. Cette fonctionnalité permet le provisionnement automatisé des VMs avec :

- **Utilisateurs personnalisés et clés SSH**
- **Configuration réseau** (DHCP ou IP statique)
- **Scripts et paquets personnalisés** via des templates YAML
- **Configurations cloud-init avancées** via le paramètre `cicustom`

Cette fonctionnalité est optionnelle et nécessite une configuration supplémentaire en raison des limitations de l'API Proxmox.

## Pourquoi cloud-init est optionnel et nécessite une configuration

### Limitations de l'API Proxmox

L'API Proxmox présente des limitations significatives concernant les téléchargements de snippets cloud-init :

- L'endpoint standard `/nodes/{node}/storage/{storage}/upload` avec `content=snippets` n'est pas supporté dans de nombreuses versions de Proxmox
- Même lorsque le stockage de snippets est configuré et activé, l'API peut retourner des erreurs HTTP 400 "bad request"
- Cette limitation affecte tous les utilisateurs Proxmox indépendamment de leur configuration de stockage

### Solution de contournement : téléchargement SSH/SFTP

En raison de ces limitations d'API, PVMSS implémente le téléchargement de snippets cloud-init via SFTP (SSH File Transfer Protocol), de manière similaire à la façon dont les fournisseurs Terraform populaires (Telmate et bpg/proxmox) gèrent cette limitation :

- **Fournisseur Telmate** : utilise SSH/SCP pour télécharger directement les snippets dans `/var/lib/vz/snippets/`
- **Fournisseur bpg** : utilise SFTP avec des comptes PAM, documenté comme "Snippets Cannot Be Uploaded by Non-PAM Accounts"

Vous pouvez en savoir plus sur cette approche dans la documentation du fournisseur Terraform bpg : <https://registry.terraform.io/providers/bpg/proxmox/latest/docs#ssh-connection>

> **Note** : PVMSS utilise la bibliothèque Go `pkg/sftp` qui implémente le protocole SFTP nativement. Aucun binaire SSH ou SFTP n'est requis dans le conteneur PVMSS - seul le fichier de clé privée doit être accessible.

Cette approche nécessite :

1. Un compte PAM dédié sur les nœuds Proxmox
2. Une paire de clés SSH (générée sur le nœud Proxmox)
3. La clé privée montée dans le conteneur PVMSS
4. Des permissions de système de fichiers appropriées pour le répertoire des snippets

## Configuration requise

Toutes les commandes ci-dessous sont exécutées sur le nœud Proxmox en tant que `root`.

### Étape 1 : Créer le compte PAM

```bash
# Créer un utilisateur dédié (pas de login shell)
useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Créer le répertoire .ssh
mkdir -p /home/pvmss-snippets/.ssh
chmod 700 /home/pvmss-snippets/.ssh
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh
```

### Étape 2 : Générer la paire de clés SSH

Générer la paire de clés directement sur le nœud Proxmox. La clé privée sera montée dans le conteneur PVMSS.

```bash
# Créer le répertoire pour les clés SSH PVMSS (sera monté dans le conteneur)
mkdir -p /etc/pvmss/keys
chmod 700 /etc/pvmss/keys

# Générer la paire de clés Ed25519 (sans passphrase pour l'automatisation)
ssh-keygen -t ed25519 -f /etc/pvmss/keys/pvmss_snippets_ed25519 -N "" -C "pvmss-snippets@pvmss"

# Définir les permissions appropriées
chmod 600 /etc/pvmss/keys/pvmss_snippets_ed25519
chmod 644 /etc/pvmss/keys/pvmss_snippets_ed25519.pub
```

### Étape 3 : Ajouter la clé publique à authorized_keys

```bash
# Copier la clé publique dans le fichier authorized_keys de l'utilisateur
cat /etc/pvmss/keys/pvmss_snippets_ed25519.pub >> /home/pvmss-snippets/.ssh/authorized_keys
chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### Étape 4 : Configurer le serveur SSH pour l'accès SFTP uniquement

Puisque l'utilisateur a `/usr/sbin/nologin` comme shell, vous devez configurer SSH pour utiliser le sous-système `internal-sftp` :

```bash
# Éditer la configuration du serveur SSH
nano /etc/ssh/sshd_config
```

Ajouter à la fin du fichier :

```ini
# Accès SFTP uniquement pour PVMSS cloud-init snippets
Match User pvmss-snippets
    ForceCommand internal-sftp
    ChrootDirectory /var/lib/vz
    AllowTcpForwarding no
    X11Forwarding no
```

Redémarrer SSH :

```bash
systemctl restart sshd
```

> **Important** : Avec `ChrootDirectory /var/lib/vz`, l'utilisateur SFTP voit `/var/lib/vz` comme racine (`/`). Le chemin dans les paramètres PVMSS devient `/snippets` au lieu de `/var/lib/vz/snippets`.

### Étape 5 : Définir les permissions pour le répertoire snippets

```bash
# Installer les outils ACL si pas déjà installés
apt install acl

# Le répertoire chroot doit appartenir à root
chown root:root /var/lib/vz
chmod 755 /var/lib/vz

# Le répertoire snippets doit être accessible en écriture par l'utilisateur
chown pvmss-snippets:pvmss-snippets /var/lib/vz/snippets
chmod 755 /var/lib/vz/snippets

# Alternativement, utiliser les ACLs pour des permissions fines
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Vérifier les permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-pvmss && rm /var/lib/vz/snippets/test-pvmss && echo "OK"
```

### Étape 6 : Exporter la clé publique du serveur pour vérification (recommandé)

Pour des connexions SFTP sécurisées, PVMSS peut vérifier l'identité du serveur Proxmox en utilisant sa clé publique d'hôte. Cela prévient les attaques Man-in-the-Middle (MitM).

```bash
# Copier la clé publique d'hôte du serveur (Ed25519 recommandé)
cp /etc/ssh/ssh_host_ed25519_key.pub /etc/pvmss/keys/proxmox_host_key.pub
chmod 644 /etc/pvmss/keys/proxmox_host_key.pub
```

> **Note** : La clé d'hôte est la clé publique du serveur, PAS la clé du client. Elle sert à vérifier l'identité du serveur lors de la connexion.

### Étape 7 : Monter les clés dans le conteneur PVMSS

⚠️ Information sensible : Le fichier de clé privée contient des identifiants qui doivent rester sécurisés.

Montez à la fois la clé privée et la clé publique d'hôte dans le conteneur :

```yaml
# docker-compose.yml
services:
  pvmss:
    image: jhmmt/pvmss:1.0
    volumes:
      - ./pvmss_snippets_ed25519:/app/pvmss_snippets_ed25519:ro
      - ./proxmox_host_key.pub:/app/proxmox_host_key.pub:ro
    # ... autre configuration
```

Ou avec `docker run` :

```bash
docker run \
  -v ./pvmss_snippets_ed25519:/app/pvmss_snippets_ed25519:ro \
  -v ./proxmox_host_key.pub:/app/proxmox_host_key.pub:ro \
  jhmmt/pvmss:1.0
```

### Étape 8 : Configurer les paramètres PVMSS

Dans votre `settings.json`, activer SFTP et configurer la connexion :

```json
{
  "cloudinit_sftp": {
    "enabled": true,
    "host": "VOTRE_IP_OU_HOSTNAME_DU_NODE_PROXMOX",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/app/pvmss_snippets_ed25519",
    "hostKeyPath": "/app/proxmox_host_key.pub",
    "snippetBaseDir": "/snippets"
  }
}
```

> **Note** : Si vous avez configuré `ChrootDirectory /var/lib/vz` à l'étape 4, utilisez `/snippets` comme `snippetBaseDir`. Si vous n'avez PAS utilisé chroot, utilisez le chemin complet `/var/lib/vz/snippets`.

### Options de configuration

| Option | Description | Exemple |
|--------|-------------|---------|
| `enabled` | Activer/désactiver les uploads SFTP | `true` |
| `host` | IP ou hostname du nœud Proxmox | `"192.168.1.100"` |
| `port` | Port SSH | `22` |
| `username` | Utilisateur PAM pour SFTP | `"pvmss-snippets"` |
| `privateKeyPath` | Chemin vers la clé privée dans le conteneur | `"/app/pvmss_snippets_ed25519"` |
| `hostKeyPath` | Chemin vers la clé publique du serveur pour vérification | `"/app/proxmox_host_key.pub"` |
| `insecureSkipHostKeyVerify` | Désactiver la vérification de clé d'hôte (NON recommandé) | `false` |
| `snippetBaseDir` | Répertoire des snippets (relatif au chroot si utilisé) | `"/snippets"` |

### Options de vérification de clé d'hôte

PVMSS requiert la vérification de clé d'hôte pour prévenir les attaques Man-in-the-Middle. Vous avez trois options :

#### Option 1 : Vérification sécurisée avec clé d'hôte (recommandé)

Fournissez la clé publique du serveur via `hostKeyPath`. C'est l'option la plus sécurisée.

```json
{
  "cloudinit_sftp": {
    "hostKeyPath": "/app/proxmox_host_key.pub"
  }
}
```

#### Option 2 : Désactiver la vérification (NON recommandé)

Si vous ne pouvez pas obtenir la clé d'hôte (ex: environnements dynamiques, tests), vous pouvez explicitement désactiver la vérification :

```json
{
  "cloudinit_sftp": {
    "insecureSkipHostKeyVerify": true
  }
}
```

> ⚠️ **Attention** : Cela désactive la protection contre les attaques Man-in-the-Middle. N'utilisez cela que dans des réseaux de confiance ou pour les tests.

#### Option 3 : Pas de SFTP (cloud-init basique uniquement)

Si vous ne pouvez pas configurer SFTP de manière sécurisée, désactivez-le et utilisez uniquement les paramètres cloud-init basiques :

```json
{
  "cloudinit_sftp": {
    "enabled": false
  }
}
```

Avec cette configuration, vous pouvez toujours utiliser `ciuser`, `cipassword`, `sshkeys`, et `ipconfig0`, mais pas les templates YAML personnalisés.

## Fonctionnement

### Gestion des templates (Admin)

1. **Créer des templates** : les admins créent des templates cloud-init dans le panneau d'administration (`/admin/cloudinit`)
2. **Validation YAML** : les templates doivent commencer par `#cloud-config` et contenir du YAML valide
3. **Activer/désactiver** : les templates peuvent être activés ou désactivés sans suppression
4. **Stockage local** : les templates sont stockés dans `settings.json` (pas sur le stockage Proxmox)

### Création de VM (Utilisateur)

1. **Sélection de template** : l'utilisateur sélectionne un template cloud-init dans le formulaire de création de VM
2. **Upload du snippet** : PVMSS télécharge le contenu YAML via SFTP vers le répertoire snippets du nœud Proxmox
3. **Configuration de la VM** : PVMSS crée la VM avec les paramètres cloud-init :
   - `ciuser`, `cipassword`, `sshkeys` pour la configuration utilisateur
   - `ipconfig0` pour la configuration réseau
   - `cicustom` pointant vers le snippet uploadé (ex: `user=local:snippets/pvmss-template.yml`)
4. **Lecteur cloud-init** : Proxmox crée une ISO contenant la configuration cloud-init
5. **Processus de démarrage** : la VM démarre et cloud-init applique la configuration

## Considérations de sécurité

- **Compte dédié** : le compte `pvmss-snippets` n'a pas d'accès shell et est restreint à SFTP
- **Permissions minimales** : le compte n'a qu'un accès en écriture au répertoire des snippets
- **Authentification par clé SSH** : aucun mot de passe n'est stocké ou transmis
- **Vérification de clé d'hôte** : PVMSS vérifie l'identité du serveur en utilisant sa clé publique, prévenant les attaques Man-in-the-Middle

### Vérification de clé d'hôte

Par défaut, PVMSS requiert la vérification de clé d'hôte pour les connexions SFTP. Cela protège contre :

- **Attaques Man-in-the-Middle (MitM)** : un attaquant ne peut pas usurper l'identité du serveur Proxmox
- **Usurpation DNS** : même si le DNS est compromis, la connexion échouera si la clé d'hôte ne correspond pas
- **Détournement réseau** : les connexions redirigées seront détectées et rejetées

Si vous voyez des erreurs concernant la vérification de clé d'hôte, vous devez soit :

1. Configurer `hostKeyPath` avec la clé publique du serveur (recommandé)
2. Définir explicitement `insecureSkipHostKeyVerify: true` (non recommandé pour la production)

## Dépannage

### Problèmes courants

1. **Permission refusée** : vérifiez les permissions du système de fichiers sur le répertoire des snippets
2. **Échec de l'authentification SSH** : vérifiez que la clé SSH est correctement configurée et accessible
3. **Snippet introuvable** : assurez-vous que le stockage supporte le type de contenu snippets
4. **Problèmes réseau** : vérifiez que PVMSS peut atteindre le nœud Proxmox sur le port 22

## Limitations

- **Connexion SSH requise** : nécessite un accès SSH aux nœuds Proxmox pour le téléchargement des snippets
- **Nœud unique** : supporte actuellement le téléchargement de snippets vers un seul nœud Proxmox (support cluster multi-nœud limité)
- **Validation des templates** : seule la syntaxe YAML est validée, pas la sémantique cloud-init
- **Pas de variables de template** : les templates sont statiques ; les valeurs spécifiques à l'utilisateur doivent être définies via les champs cloud-init basiques (utilisateur, mot de passe, clés SSH)

### Échec de l'upload SFTP

1. **Vérifier le statut SFTP** : allez sur `/admin/cloudinit` pour voir le statut de la configuration SFTP
2. **Vérifier la clé privée** : assurez-vous que le fichier de clé existe et a les bonnes permissions (600)
3. **Tester la connexion SFTP manuellement** :

   ```bash
   # Tester la connexion SFTP (pas SSH - l'utilisateur n'a pas de shell)
   sftp -i /chemin/vers/pvmss_snippets_ed25519 pvmss-snippets@VOTRE_HOST_PROXMOX
   ```

4. **Vérifier les permissions du répertoire snippets** : l'utilisateur doit avoir un accès en écriture à `/var/lib/vz/snippets`

5. **Erreur "packet too long"** : Cette erreur survient quand le serveur SSH envoie des données inattendues avant la négociation SFTP. Assurez-vous que le bloc `Match User` avec `ForceCommand internal-sftp` est correctement configuré dans `/etc/ssh/sshd_config`.

6. **Vérifier la config SSH** : Assurez-vous que le bloc `Match User` est à la **fin** du fichier `sshd_config`.

### Cloud-init non appliqué

1. **Vérifier le contenu du template** : les templates doivent commencer par `#cloud-config`
2. **Vérifier le lecteur cloud-init de la VM** : la VM doit avoir `ide2` configuré comme cloudinit
3. **Consulter les logs Proxmox** : `/var/log/pve/tasks/` contient les logs des tâches
4. **Console VM** : vérifiez `/var/log/cloud-init.log` à l'intérieur de la VM

## Approches alternatives

Si vous ne pouvez pas utiliser l'approche SSH :

1. **Snippets manuels** : créez manuellement les snippets sur les nœuds Proxmox et référencez-les dans PVMSS (ajoutez le chemin vers le snippet dans le paramètre cicustom, par exemple `user=local:snippets/mon-modele.yaml`)
2. **Cloud-init de base** : utilisez uniquement les paramètres cloud-init de base (ciuser, cipassword, ipconfig0)
3. **Configuration externe** : utilisez des outils de gestion de configuration externes comme Ansible

## Exemples de Templates

### Configuration utilisateur basique

```yaml
#cloud-config
users:
  - name: admin
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ssh-ed25519 AAAAC3... utilisateur@exemple.com
```

### Installation de paquets

```yaml
#cloud-config
package_update: true
package_upgrade: true
packages:
  - vim
  - htop
  - curl
  - git
```

### Exécution de scripts personnalisés

```yaml
#cloud-config
runcmd:
  - echo "Bonjour depuis cloud-init" > /tmp/bonjour.txt
  - systemctl enable --now docker
```
