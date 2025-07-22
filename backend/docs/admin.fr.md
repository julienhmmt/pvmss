# Guide Administrateur PVMSS

## Vue d'Ensemble

Ce guide couvre toutes les fonctionnalités administratives et workflows disponibles dans PVMSS (Proxmox Virtual Machine Self-Service), incluant la configuration système, la gestion des utilisateurs et la maintenance.

## Fonctionnalités Administratives

### Configuration Système

- **Gestion des ISO** : Contrôler quelles images ISO sont disponibles aux utilisateurs pour la création de VM
- **Gestion du Stockage** : Voir et gérer les ressources de stockage disponibles
- **Ponts Réseau (VMBR)** : Configurer les ponts réseau disponibles pour la mise en réseau des VM
- **Limites de Ressources** : Définir des limites sur les ressources CPU, mémoire et stockage
- **Paramètres de Sécurité** : Gérer l'authentification et le contrôle d'accès

### Gestion des Utilisateurs

- **Contrôle d'Accès** : Configurer les permissions utilisateur et l'authentification
- **Quotas de Ressources** : Définir des limites de ressources par utilisateur
- **Audit et Logs** : Surveiller les activités utilisateur et les changements système

### Surveillance et Maintenance

- **Santé Système** : Surveiller le statut et les performances de l'application PVMSS
- **Gestion des Logs** : Examiner les logs d'application pour le dépannage
- **Configuration de Sauvegarde** : S'assurer que les paramètres critiques sont sauvegardés
- **Mises à Jour** : Maintenir PVMSS à jour avec les derniers correctifs de sécurité

### Guide de Démarrage

1. Accéder au panneau d'administration sur `/admin` (authentification requise)
2. Examiner et configurer les images ISO pour la création de VM utilisateur
3. Configurer les ponts réseau et les options de stockage
4. Définir des limites de ressources appropriées basées sur votre infrastructure
5. Surveiller régulièrement les logs système pour détecter tout problème

### Bonnes Pratiques de Sécurité

- Mettre à jour régulièrement les mots de passe administrateur avec le générateur de hachage fourni
- Examiner périodiquement les logs d'accès utilisateur
- Maintenir l'application PVMSS à jour
- Surveiller l'accès et l'utilisation de l'API Proxmox
- Utiliser HTTPS dans les environnements de production
- Empêcher les utilisateurs de modifier les paramètres BIOS des machines virtuelles
