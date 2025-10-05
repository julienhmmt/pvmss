# Guide de l'utilisateur de PVMSS

PVMSS (Proxmox Virtual Machine Self-Service) est une application web intuitive permettant de créer, gérer et accéder aux consoles des machines virtuelles hébergées sur un serveur Proxmox Virtual Environment, de manière simplifiée.

## Guide de démarrage rapide

1. **Connexion à l'application** : Connectez-vous à PVMSS à l'aide de vos identifiants pour accéder aux fonctionnalités de création et de gestion des machines virtuelles.
2. **Recherche de machines virtuelles** : Utilisez la fonction de recherche pour localiser une machine virtuelle spécifique par son nom ou son VMID et consulter ses détails.
3. **Création d'une machine virtuelle** : Cliquez sur le bouton "Créer une VM" pour ouvrir le formulaire de configuration, puis renseignez les paramètres requis.
4. **Accès à la console** : Une fois la machine virtuelle créée et démarrée, cliquez sur le bouton "Console" pour vous connecter à son interface graphique via le client web noVNC intégré.
5. **Gestion du profil** : Accédez à votre profil pour consulter et modifier votre description et gérer les tags qui vous sont assignés.

## Fonctionnalités principales

### Création d'une machine virtuelle

Pour créer une machine virtuelle, accédez au formulaire de configuration via le bouton "Créer une VM" après vous être connecté à PVMSS. Les paramètres suivants doivent être configurés :

- **Nœud** : Sélectionnez le nœud Proxmox sur lequel la VM sera créée (parmi les nœuds disponibles configurés par les administrateurs).
- **Nom et description** : Saisissez un nom unique (caractères alphanumériques, tirets et underscores uniquement) et une description pour identifier votre machine virtuelle.
- **Système d'exploitation** : Sélectionnez une image ISO parmi une liste prédéfinie par les administrateurs pour installer le système d'exploitation.
- **Ressources** : Configurez les ressources nécessaires :
  - **Cœurs CPU** : Nombre de cœurs processeur (dans les limites fixées par les administrateurs)
  - **Mémoire (RAM)** : Quantité de mémoire en Mo (dans les limites fixées par les administrateurs)
  - **Taille du disque** : Capacité de stockage en Go (dans les limites fixées par les administrateurs)
  - **Pont réseau** : Sélectionnez le pont réseau (VMBR) pour la connectivité réseau
- **Tags** : Ajoutez des tags prédéfinis pour organiser et faciliter la recherche de vos machines virtuelles.

**Notes importantes :**

- Vous ne pouvez créer qu'une seule machine à la fois.
- Les limites de ressources par machine virtuelle (CPU, RAM, disque) sont imposées par les administrateurs.
- Vous ne pouvez pas modifier les ressources d'une machine virtuelle existante après sa création.

### Recherche d'une machine virtuelle

Utilisez la fonction de recherche pour localiser une machine virtuelle par son **nom** ou son **VMID** (identifiant unique attribué à chaque machine virtuelle par Proxmox). La recherche est insensible à la casse et supporte la correspondance partielle.

Une liste de résultats s'affichera en fonction des critères saisis, affichant :

- Le VMID
- Le nom de la VM
- Le node sur lequel la VM est hébergée
- L'état actuel (en cours d'exécution, arrêté, etc.)
- Le bouton "Détails de la VM" (pour consulter les informations complètes de la machine virtuelle et accéder aux fonctionnalités de gestion avancées).

### Gestion d'une machine virtuelle

La page de détails de la VM fournit des capacités complètes de gestion et de surveillance :

#### Actions de contrôle

- **Démarrer** : Allumer la machine virtuelle
- **Console** : Ouvrir la console noVNC intégrée dans une nouvelle fenêtre pour un accès graphique
- **Redémarrer** : Redémarrer la machine virtuelle
- **Éteindre** : Arrêt gracieux (envoie un signal d'arrêt ACPI)
- **Arrêter** : Forcer l'arrêt de la machine virtuelle (arrêt immédiat)
- **Reset** : Forcer la réinitialisation de la machine virtuelle
- **Actualiser** : Rafraîchir les informations de la VM (invalidation du cache)
- **Supprimer** : Supprimer définitivement la machine virtuelle (nécessite une confirmation)

### Détails de configuration

Consultez les informations en temps réel sur votre machine virtuelle :

- **État** : État actuel (en cours d'exécution, arrêté, etc.)
- **Temps de fonctionnement** : Durée pendant laquelle la VM est en cours d'exécution
- **Utilisation CPU** : Pourcentage d'utilisation du processeur actuel
- **Utilisation mémoire** : Utilisation actuelle de la RAM (utilisé/total)
- **Utilisation disque** : Espace de stockage utilisé par la VM
- **Réseau** : Affichage des paramètres réseau de la VM

Consultez les informations de configuration détaillées :

- Nom et description de la VM
- Emplacement du nœud
- Allocation des cœurs CPU et de la mémoire
- Configuration du disque
- Paramètres réseau
- Tags assignés

### Gestion du profil

Vous pouvez modifier certaines propriétés de la VM :

- **Description** : Mettre à jour la description de la VM
- **Tags** : Ajouter ou supprimer des tags pour une meilleure organisation

**Note** : Les ressources matérielles (CPU, RAM, disque) et le choix du pont réseau ne peuvent pas être modifiées après la création de la VM.

### Accès à la console

PVMSS fournit un accès console intégré à vos machines virtuelles via noVNC, un client VNC basé sur le web.

1. Connectez-vous à l'application PVMSS
2. Accédez à la page de détails de la VM (soit via la fonction de recherche, soit depuis votre profil)
3. Assurez-vous que la VM est en cours d'exécution (démarrez-la si nécessaire)
4. Cliquez sur le bouton "Console"

#### Fonctionnalités de la console

- **Support complet du clavier et de la souris** : Interagissez avec votre VM comme si vous utilisiez un moniteur physique
- **Indicateurs de connexion** : Retour visuel montrant l'état de la connexion
- **Reconnexion automatique** : La console tente de se reconnecter si la connexion est perdue

#### Dépannage de la console

Si vous rencontrez des problèmes de connexion à la console :

- Assurez-vous que la VM est en cours d'exécution (la console ne fonctionne que pour les VM démarrées)
- Déconnectez-vous de l'application PVMSS et reconnectez-vous
- Actualisez la fenêtre de la console si la connexion est perdue
- Contactez votre administrateur si les problèmes persistent

**Note** : La session console est authentifiée à l'aide de vos identifiants PVMSS et fournit un accès sécurisé à l'interface graphique de votre VM.

## Bonnes pratiques

- **Arrêt approprié** : Utilisez toujours le bouton "Éteindre" (arrêt gracieux) plutôt que "Arrêter" lorsque c'est possible pour éviter toute perte de données et garantir que le système d'exploitation s'arrête correctement.
- **Convention de nommage** : Utilisez des noms clairs et descriptifs conformes aux normes de votre organisation pour vos machines virtuelles. Utilisez uniquement des caractères alphanumériques, des tirets et des underscores.
- **Planification des ressources** : Planifiez vos besoins en ressources avant de créer une VM. Contactez votre administrateur si vous avez besoin de ressources au-delà des limites configurées.
- **Organisation par tags** : Utilisez les tags de manière cohérente pour organiser vos VMs et les rendre plus faciles à trouver.
- **Sécurité de la console** : Fermez la fenêtre de la console lorsqu'elle n'est pas utilisée pour libérer des ressources.
- **Sécurité des identifiants** : Ne partagez jamais vos identifiants de connexion afin de garantir la sécurité de votre compte et de vos machines virtuelles.
- **Surveillance régulière** : Vérifiez régulièrement l'utilisation des ressources de votre VM pour vous assurer qu'elle fonctionne efficacement.

## Support

L'application PVMSS est gérée par l'équipe informatique de votre organisation. Pour toute assistance, contactez votre administrateur dans les cas suivants :

- **Perte de mot de passe** : Vous pouvez changer votre mot de passe via le bouton "Modifier le mot de passe" dans votre profil. Votre administrateur peut réinitialiser votre mot de passe depuis le noeud Proxmox si vous avez perdu votre mot de passe.
- **Augmentation des limites de ressources** : Si vous avez besoin de plus de CPU, RAM ou disque que les limites configurées ne le permettent, contactez votre administrateur.
- **Difficultés lors de la création d'une machine virtuelle** : Problèmes avec la création, la configuration ou le déploiement de VM, contactez votre administrateur.
- **Problèmes d'accès à la console** : Problèmes de connexion ou d'utilisation de la console VM, contactez votre administrateur.
- **Problèmes de permissions** : Si vous ne pouvez pas accéder à certaines fonctionnalités ou VMs, contactez votre administrateur.
- **Problèmes techniques** : Toute erreur, bug ou comportement inattendu dans l'application, contactez votre administrateur.
- **Demandes de fonctionnalités** : Suggestions pour de nouvelles ISOs, ponts réseau ou autres ressources, contactez votre administrateur.

## Limites connues

L'application PVMSS ne prend actuellement pas en charge :

- **Modification des ressources** : Vous ne pouvez pas modifier les ressources de la machine virtuelle (CPU, mémoire, stockage, pont réseau) après sa création. Si vous devez modifier les ressources, vous devez créer une nouvelle VM et migrer vos données.
- **Gestion des mots de passe** : Vous ne pouvez pas modifier votre nom d'utilisateur et votre mot de passe via l'interface. Contactez votre administrateur pour la réinitialisation des mots de passe.
- **Conteneurs LXC** : Seules les machines virtuelles KVM/QEMU sont prises en charge. La création de conteneurs LXC n'est pas disponible.
- **Snapshots** : La création et la gestion de snapshots de VM ne sont pas disponibles via PVMSS.
- **Sauvegardes** : Les opérations de sauvegarde et de restauration de VM doivent être effectuées par les administrateurs directement via Proxmox.
- **Migration en direct** : Le déplacement de VMs entre nœuds n'est pas disponible via PVMSS.
- **Mise en réseau avancée** : Seule l'assignation de pont réseau de base est prise en charge. Les fonctionnalités de mise en réseau avancées (VLANs, règles de pare-feu, etc.) doivent être configurées par les administrateurs.
- **Accès direct à Proxmox** : PVMSS est conçu comme une interface simplifiée et ne fournit pas l'accès à toutes les fonctionnalités de Proxmox.

## Sécurité et confidentialité

- Les sessions console sont authentifiées et basées sur les sessions.
- Chaque utilisateur ne peut voir et gérer que ses propres machines virtuelles.
- L'accès administrateur est séparé de l'accès utilisateur avec une authentification supplémentaire.

## Astuces et conseils

- **Démarrage rapide de VM** : Utilisez la page de recherche pour un accès rapide au démarrage/arrêt des VMs sans ouvrir la page de détails.
- **Favoris du navigateur** : Ajoutez l'URL de PVMSS et les pages de détails de VMs spécifiques à vos favoris pour un accès rapide.
- **Fenêtres multiples** : Vous pouvez ouvrir plusieurs fenêtres de console VM simultanément pour gérer plusieurs VMs.
- **Changement de langue** : L'application détecte automatiquement la préférence de langue de votre navigateur. Modifiez les paramètres de langue de votre navigateur pour basculer entre le français et l'anglais.
- **Raccourcis clavier** : La plupart des navigateurs modernes prennent en charge les raccourcis clavier dans la fenêtre de console (Ctrl+C, Ctrl+V pour les opérations du presse-papiers).
