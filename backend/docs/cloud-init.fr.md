# Support cloud-init dans PVMSS

## Vue d'ensemble

PVMSS fournit un support cloud-init pour les VMs Proxmox, permettant aux utilisateurs d'appliquer des configurations cloud-init avancées lors de la création de VM. Cette fonctionnalité est optionnelle et nécessite une configuration supplémentaire en raison des limitations de l'API Proxmox.

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

### Étape 4 : Définir les permissions pour le répertoire snippets

```bash
# Installer les outils ACL si pas déjà installés
apt install acl

# Pour le stockage 'local', les snippets sont typiquement dans /var/lib/vz/snippets
setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Vérifier les permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test-pvmss && rm /var/lib/vz/snippets/test-pvmss && echo "OK"
```

### Étape 5 : Monter la clé privée dans le conteneur PVMSS

Ajouter un montage de volume dans votre Docker Compose ou commande Docker run :

```yaml
# docker-compose.yml
services:
  pvmss:
    image: pvmss:latest
    volumes:
      - ./:/etc/pvmss/keys:ro
    # ... autre configuration
```

Ou avec `docker run` :

```bash
docker run -v /etc/pvmss/keys:/etc/pvmss/keys:ro ... pvmss:latest
```

### Étape 6 : Configurer les paramètres PVMSS

Dans votre `settings.json`, activer SFTP et configurer la connexion :

```json
{
  "cloudinit_sftp": {
    "enabled": true,
    "host": "VOTRE_IP_OU_HOSTNAME_DU_NODE_PROXMOX",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/etc/pvmss/keys/pvmss_snippets_ed25519",
    "snippetBaseDir": "/var/lib/vz/snippets"
  }
}
```

## Fonctionnement

1. **Sélection de modèle** : l'utilisateur sélectionne un modèle cloud-init dans le formulaire de création de VM
2. **Téléchargement du snippet** : PVMSS télécharge le contenu YAML via SFTP vers le nœud Proxmox
3. **Configuration de la VM** : PVMSS définit le paramètre `cicustom` pour référencer le snippet téléversé
4. **Processus de démarrage** : Proxmox crée un disque cloud-init avec la configuration personnalisée

## Considérations de sécurité

- **Compte dédié** : le compte `pvmss-snippets` n'a pas d'accès shell et est restreint à SFTP
- **Permissions minimales** : le compte n'a qu'un accès en écriture au répertoire des snippets
- **Authentification par clé SSH** : aucun mot de passe n'est stocké ou transmis

## Dépannage

### Problèmes courants

1. **Permission refusée** : vérifiez les permissions du système de fichiers sur le répertoire des snippets
2. **Échec de l'authentification SSH** : vérifiez que la clé SSH est correctement configurée et accessible
3. **Snippet introuvable** : assurez-vous que le stockage supporte le type de contenu snippets
4. **Problèmes réseau** : vérifiez que PVMSS peut atteindre le nœud Proxmox sur le port 22

## Limitations

- **Connexion SSH requise** : nécessite un accès SSH aux nœuds Proxmox pour le téléchargement des snippets
- **Nœud unique** : supporte actuellement le téléchargement de snippets vers un seul nœud Proxmox

## Approches alternatives

Si vous ne pouvez pas utiliser l'approche SSH :

1. **Snippets manuels** : créez manuellement les snippets sur les nœuds Proxmox et référencez-les dans PVMSS (ajoutez le chemin vers le snippet dans le paramètre cicustom, par exemple `user=local:snippets/mon-modele.yaml`)
2. **Cloud-init de base** : utilisez uniquement les paramètres cloud-init de base (ciuser, cipassword, ipconfig0)
3. **Configuration externe** : utilisez des outils de gestion de configuration externes comme Ansible
