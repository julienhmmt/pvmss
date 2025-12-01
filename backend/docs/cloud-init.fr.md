# Support Cloud-Init dans PVMSS

## Vue d'ensemble

PVMSS fournit un support cloud-init pour les VMs Proxmox, permettant aux utilisateurs d'appliquer des configurations cloud-init avancées lors de la création de VM. Cette fonctionnalité est optionnelle et nécessite une configuration supplémentaire en raison des limitations de l'API Proxmox.

## Pourquoi Cloud-Init est optionnel et nécessite une configuration

### Limitations de l'API Proxmox

L'API Proxmox présente des limitations significatives concernant les téléchargements de snippets cloud-init :

- L'endpoint standard `/nodes/{node}/storage/{storage}/upload` avec `content=snippets` n'est pas supporté dans de nombreuses versions de Proxmox
- Même lorsque le stockage de snippets est configuré et activé, l'API peut retourner des erreurs HTTP 400 "bad request"
- Cette limitation affecte tous les utilisateurs Proxmox indépendamment de leur configuration de stockage

### Solution de contournement : Téléchargement SSH/SFTP

En raison de ces limitations d'API, PVMSS implémente le téléchargement de snippets cloud-init via SSH/SFTP, de manière similaire à la façon dont les fournisseurs Terraform populaires (Telmate et bpg/proxmox) gèrent cette limitation :

- **Fournisseur Telmate** : Utilise SSH/SCP pour télécharger directement les snippets dans `/var/lib/vz/snippets/`
- **Fournisseur bpg** : Utilise SFTP avec des comptes PAM, documenté comme "Snippets Cannot Be Uploaded by Non-PAM Accounts"

Cette approche nécessite :

1. Un compte PAM dédié sur les nœuds Proxmox
2. Authentification par clé SSH
3. Permissions de système de fichiers appropriées pour le répertoire des snippets

## Configuration requise

### 1. Créer un compte PAM sur chaque nœud Proxmox

```bash
# Créer un utilisateur dédié (pas de login shell)
sudo useradd --create-home --shell /usr/sbin/nologin pvmss-snippets

# Créer le répertoire .ssh et définir les permissions
sudo mkdir -p /home/pvmss-snippets/.ssh
sudo chmod 700 /home/pvmss-snippets/.ssh
sudo chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh

# Ajouter la clé publique SSH (remplacer par votre clé publique réelle)
echo 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI... votre-clé-publique-ici' | sudo tee /home/pvmss-snippets/.ssh/authorized_keys
sudo chmod 600 /home/pvmss-snippets/.ssh/authorized_keys
sudo chown pvmss-snippets:pvmss-snippets /home/pvmss-snippets/.ssh/authorized_keys
```

### 2. Définir les permissions du système de fichiers

```bash
# Pour le stockage 'local', les snippets sont typiquement dans /var/lib/vz/snippets
sudo setfacl -m u:pvmss-snippets:rwx /var/lib/vz/snippets
sudo setfacl -R -m u:pvmss-snippets:rwX /var/lib/vz/snippets

# Vérifier les permissions
sudo -u pvmss-snippets ls -la /var/lib/vz/snippets
```

### 3. Tester l'accès SFTP

```bash
# Tester depuis le serveur PVMSS
sftp -oIdentityFile=/chemin/vers/clé-privée pvmss-snippets@<ip-du-nœud>
cd /var/lib/vz/snippets
put test.yml
```

## Configuration PVMSS

### Stockage de la clé SSH

Stockez la clé SSH privée de manière sécurisée sur le serveur PVMSS :

```bash
# Créer le répertoire pour les clés SSH
sudo mkdir -p /etc/pvmss/keys
sudo chmod 700 /etc/pvmss/keys

# Placer la clé privée
sudo cp /chemin/vers/clé-privée /etc/pvmss/keys/pvmss_snippets_ed25519
sudo chmod 600 /etc/pvmss/keys/pvmss_snippets_ed25519
sudo chown pvmss:pvmss /etc/pvmss/keys/pvmss_snippets_ed25519
```

### Paramètres PVMSS

Ajoutez la configuration suivante à vos paramètres PVMSS :

```json
{
  "cloudInitSFTP": {
    "enabled": true,
    "host": "pve-node-1",
    "port": 22,
    "username": "pvmss-snippets",
    "privateKeyPath": "/etc/pvmss/keys/pvmss_snippets_ed25519",
    "snippetBaseDir": "/var/lib/vz/snippets"
  }
}
```

## Fonctionnement

1. **Sélection de modèle** : L'utilisateur sélectionne un modèle cloud-init dans le formulaire de création de VM
2. **Téléchargement du snippet** : PVMSS télécharge le contenu YAML via SFTP vers le nœud Proxmox
3. **Configuration de la VM** : PVMSS définit le paramètre `cicustom` pour référencer le snippet téléchargé
4. **Processus de démarrage** : Proxmox crée un disque cloud-init avec la configuration personnalisée

## Considérations de sécurité

- **Compte dédié** : Le compte `pvmss-snippets` n'a pas d'accès shell et est restreint à SFTP
- **Permissions minimales** : Le compte n'a qu'un accès en écriture au répertoire des snippets
- **Authentification par clé SSH** : Aucun mot de passe n'est stocké ou transmis
- **Considération Chroot** : Pour une sécurité supplémentaire, vous pouvez configurer SSH chroot pour le compte

## Dépannage

### Problèmes courants

1. **Permission refusée** : Vérifiez les permissions du système de fichiers sur le répertoire des snippets
2. **Échec de l'authentification SSH** : Vérifiez que la clé SSH est correctement configurée et accessible
3. **Snippet introuvable** : Assurez-vous que le stockage supporte le type de contenu snippets
4. **Problèmes réseau** : Vérifiez que PVMSS peut atteindre le nœud Proxmox sur le port 22

### Commandes de débogage

```bash
# Vérifier les permissions utilisateur
sudo -u pvmss-snippets touch /var/lib/vz/snippets/test

# Vérifier la configuration SSH
sudo sshd -T | grep -i match

# Tester manuellement la connexion SFTP
sftp -oIdentityFile=/etc/pvmss/keys/pvmss_snippets_ed25519 -oPort=22 pvmss-snippets@<ip-du-nœud>
```

## Limitations

- **Nœud unique** : Supporte actuellement le téléchargement de snippets vers un seul nœud Proxmox
- **Type de stockage** : Fonctionne avec le stockage local ; le stockage partagé nécessite une configuration supplémentaire
- **Version Proxmox** : Peut ne pas fonctionner sur de très anciennes versions de Proxmox sans support des snippets

## Approches alternatives

Si vous ne pouvez pas utiliser l'approche SSH/SFTP :

1. **Snippets manuels** : Créez manuellement les snippets sur les nœuds Proxmox et référencez-les dans PVMSS
2. **Cloud-Init de base** : Utilisez uniquement les paramètres cloud-init de base (ciuser, cipassword, ipconfig0)
3. **Configuration externe** : Utilisez des outils de gestion de configuration externes comme Ansible

## Améliorations futures

Améliorations potentielles pour le support cloud-init :

- Support multi-nœuds avec sélection automatique de nœud
- Support de stockage partagé pour les snippets à l'échelle du cluster
- Intégration avec des systèmes de gestion de secrets
- Scripts de provisionnement automatique de comptes
