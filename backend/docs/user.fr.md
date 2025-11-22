# Guide de l'utilisateur de PVMSS

PVMSS (Proxmox Virtual Machine Self-Service) est une application web intuitive permettant de créer, gérer et accéder aux consoles des machines virtuelles hébergées sur un serveur Proxmox Virtual Environment, de manière simplifiée.

## Table des matières

- [Guide de démarrage rapide](#guide-de-démarrage-rapide)
- [Fonctionnalités principales](#fonctionnalités-principales)
- [Bonnes pratiques](#bonnes-pratiques)
- [Support](#support)
- [FAQ](#faq)
- [PVMSS vs Proxmox](#pvmss-vs-proxmox-ve--que-faire-où-)
- [Limites connues](#limites-connues)
- [Sécurité et confidentialité](#sécurité-et-confidentialité)
- [Astuces et conseils](#astuces-et-conseils)

## Guide de démarrage rapide

1. **Connexion à l'application** : Connectez-vous à PVMSS à l'aide de vos identifiants pour accéder aux fonctionnalités de création et de gestion des machines virtuelles.
2. **Recherche de machines virtuelles** : Utilisez la fonction de recherche pour localiser une machine virtuelle spécifique par son nom ou son VMID et consulter ses détails.
3. **Création d'une machine virtuelle** : Cliquez sur le bouton "Créer une VM" pour ouvrir le formulaire de configuration, puis renseignez les paramètres requis.
4. **Accès à la console** : Une fois la machine virtuelle créée et démarrée, cliquez sur le bouton "Console" pour vous connecter à son interface graphique via le client web noVNC intégré.
5. **Gestion du profil** : Accédez à votre profil pour consulter la liste de vos VMs, voir combien sont en cours d'exécution ou arrêtées, et changer votre mot de passe si nécessaire.

## Fonctionnalités principales

### Création d'une machine virtuelle

Pour créer une machine virtuelle, accédez au formulaire de configuration via le bouton "Créer une VM" après vous être connecté à PVMSS. Les paramètres suivants doivent être configurés :

- **Nœud** : Sélectionnez le nœud Proxmox sur lequel la VM sera créée (parmi les nœuds disponibles configurés par les administrateurs). Certains nœuds peuvent être désactivés s'ils ont atteint les limites définies par l'administrateur.
- **Nom et description** : Saisissez un nom unique (caractères alphanumériques, tirets et underscores uniquement) et une description pour identifier votre machine virtuelle.
- **Système d'exploitation** : Sélectionnez une image ISO parmi une liste prédéfinie par les administrateurs pour installer le système d'exploitation.
- **Ressources** : Configurez les ressources nécessaires :
  - **CPU** : Nombre de sockets et de cœurs CPU (dans les limites fixées par les administrateurs)
  - **Mémoire (RAM)** : Quantité de mémoire en Mo ou en Go (dans les limites fixées par les administrateurs)
  - **Disques** : Taille du disque principal en Go et, si les administrateurs l'autorisent, un ou plusieurs disques de données supplémentaires
  - **Type de bus disque** : Bus de stockage utilisé pour les disques (VirtIO, SCSI, SATA, IDE), ce qui peut avoir un impact sur les performances et sur le nombre maximal de disques
- **Stockage** : Sélectionnez le stockage sur lequel les disques de la VM seront créés, parmi les stockages activés par les administrateurs.
- **Réseau** : Configurez une ou plusieurs cartes réseau (en fonction de la configuration définie par l'administrateur) :
  - **Pont réseau** : Sélectionnez le pont réseau (VMBR) pour la connectivité
  - **Modèle de carte réseau** : Choisissez le modèle (VirtIO, E1000, E1000E, RTL8139, VMXNet3)
  - **Adresse MAC** : Facultatif, vous pouvez saisir une adresse MAC ou laisser PVMSS en générer une automatiquement
- **Firmware et sécurité** :
  - **Démarrage EFI** : Active le firmware UEFI (généralement activé par défaut pour les systèmes modernes)
  - **TPM (Trusted Platform Module)** : Permet d'activer un TPM v2.0 pour les systèmes qui l'exigent (par exemple Windows 11)
- **Démarrage** : Choisissez si la VM doit être démarrée automatiquement après sa création.
- **Tags** : Ajoutez des tags prédéfinis pour organiser et faciliter la recherche de vos machines virtuelles.

**Notes importantes :**

- Vous ne pouvez créer qu'une seule machine à la fois.
- Les limites de ressources par machine virtuelle (CPU, RAM, disque) sont imposées par les administrateurs.
- Des quotas supplémentaires peuvent s'appliquer, comme un nombre maximum de VMs par utilisateur ou des plafonds globaux par nœud. Si vous atteignez ces limites, vous ne pourrez plus créer de nouvelles VMs tant qu'un administrateur n'aura pas ajusté la configuration.

### Recherche d'une machine virtuelle

Utilisez la fonction de recherche pour localiser une machine virtuelle par son **nom**, son **VMID** ou ses **tags** associés (par exemple un environnement ou un projet). La recherche est insensible à la casse et supporte la correspondance partielle.

Une liste de résultats s'affichera en fonction des critères saisis, affichant :

- Le VMID
- Le nom de la VM
- Le nœud sur lequel la VM est hébergée
- Les tags associés à la VM (hors tag interne `pvmss`)
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

#### QEMU guest agent

Sur la page *Détails de la VM*, un petit badge intitulé **“QEMU guest agent”** indique l'état connu de l'agent QEMU à l'intérieur de la machine virtuelle :

- **Disponible** : PVMSS a récemment reçu des données de l'agent (par exemple les adresses IP). Les arrêts gracieux devraient fonctionner.
- **Indisponible** : L'agent invité n'est pas installé, n'est pas en cours d'exécution, ou n'a pas répondu à temps. Dans ce cas :
  - L'action **Éteindre** peut échouer rapidement avec un message clair suggérant d'utiliser **Arrêter**.
  - Vous devriez préférer **Arrêter** si la VM ne réagit pas à Éteindre.
- **Inconnu** : L'agent n'a pas été interrogé récemment, ou la VM est arrêtée. PVMSS tentera un bref contrôle de l'agent la prochaine fois que vous utiliserez une action qui en dépend.
- **Hors ligne (PVMSS)** : L'application est en **mode hors ligne** et n'appelle pas l'API Proxmox. Dans cet état, les actions dépendant de l'agent QEMU (comme l'arrêt gracieux) ne sont pas disponibles.

**Important** : Préférez toujours **Éteindre** (arrêt gracieux) plutôt que **Arrêter** (arrêt forcé). Si vous voyez des messages répétés sur l'agent QEMU invité indisponible, contactez votre administrateur ou installez/activez l'agent à l'intérieur de la VM.

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

Vous pouvez modifier certaines propriétés de la VM depuis la page *Détails de la VM* :

- **Description** : Mettre à jour la description de la VM (texte simple ou Markdown léger, selon la configuration choisie par l'administrateur)
- **Tags** : Ajouter ou supprimer des tags pour une meilleure organisation

Votre **profil** affiche un résumé de vos VMs (nombre total, en cours d'exécution, arrêtées) et fournit un formulaire sécurisé pour changer votre mot de passe.

### Modification des ressources d'une machine virtuelle

En plus de la description et des tags, vous pouvez modifier certaines ressources d'une VM existante depuis la page *Détails de la VM*, à condition que la VM soit **arrêtée** :

- **CPU** : Nombre de sockets et de cœurs (dans les limites définies par les administrateurs)
- **Mémoire (RAM)** : Mémoire allouée en Mo/Go (dans les limites définies par les administrateurs)
- **Cartes réseau** :
  - Pont utilisé par chaque carte réseau
  - Modèle de carte (VirtIO, E1000, E1000E, RTL8139, VMXNet3)
  - Adresse MAC de chaque carte (optionnelle)
- **CD-ROM / ISO** : Image ISO chargée dans le lecteur CD-ROM virtuel, ou éjection de l'ISO actuelle

Certaines opérations restent restreintes et peuvent nécessiter la création d'une nouvelle VM puis la copie des données manuellement, par exemple :

- Modifier la taille des disques ou le nombre de disques au-delà de ce que les administrateurs autorisent
- Migrer vers un autre stockage lorsque ce n'est pas pris en charge automatiquement par Proxmox
- Effectuer des changements structurels qui ne sont pas exposés dans l'interface PVMSS

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

- **Arrêt approprié** : Utilisez toujours le bouton "Éteindre" (arrêt gracieux) plutôt que "Arrêter" lorsque c'est possible pour éviter toute perte de données et garantir que le système d'exploitation s'arrête correctement. N'utilisez "Arrêter" ou "Reset" qu'en dernier recours si la VM ne répond plus.
- **Convention de nommage** : Utilisez des noms clairs et descriptifs conformes aux normes de votre organisation pour vos machines virtuelles. Un schéma courant est `equipe-env-rôle` (par exemple `ml-prod-api`, `etudiants-dev-lab1`). Utilisez uniquement des caractères alphanumériques, des tirets et des underscores.
- **Planification des ressources** : Planifiez vos besoins en ressources avant de créer une VM. Contactez votre administrateur si vous avez besoin de ressources au-delà des limites configurées.
- **Organisation par tags** : Utilisez les tags de manière cohérente pour organiser vos VMs et les rendre plus faciles à trouver. Privilégiez des tags structurés comme `env:prod`, `env:test`, `team:ml`, `project:monapp` ou `promo:2025` plutôt que du texte libre.
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

## FAQ

- **Pourquoi le bouton "Créer une VM" est-il grisé ou désactivé ?**  
  Vous avez peut-être atteint une limite de ressources ou de quota (par exemple nombre maximum de VMs par utilisateur ou par nœud), le nœud ou le stockage sélectionné peut être désactivé, ou l'application peut fonctionner en mode hors‑ligne. Vérifiez le message d'erreur affiché sur la page et contactez votre administrateur si nécessaire.
- **Pourquoi je ne peux pas sélectionner plus de CPU, de RAM ou de disque pour ma VM ?**  
  Les limites par VM sont définies par les administrateurs dans PVMSS. Si vous avez besoin de plus de ressources que la plage autorisée, vous devez demander une augmentation à votre administrateur.
- **Pourquoi certains nœuds sont-ils indisponibles ou grisés dans le formulaire de création de VM ?**  
  Les administrateurs peuvent avoir désactivé certains nœuds, ou le nœud peut avoir atteint ses limites agrégées configurées. Dans ce cas, vous devez choisir un autre nœud ou attendre que votre administrateur libère ou augmente les ressources.
- **Que signifie le mode hors‑ligne pour moi ?**  
  En mode hors‑ligne, PVMSS n'appelle pas l'API Proxmox. Vous pouvez éventuellement vous connecter et voir des informations mises en cache, mais les opérations comme la création de nouvelles VMs ou la modification des ressources sont temporairement indisponibles tant que les administrateurs n'ont pas rétabli la connectivité.
- **Que faire si la fenêtre de console reste noire ou ne se connecte pas ?**  
  Vérifiez d'abord que la VM est en cours d'exécution, puis actualisez la fenêtre de console ou déconnectez-vous/reconnectez-vous à l'application. Si le problème persiste, contactez votre administrateur en indiquant le VMID et l'heure approximative du problème.
- **Puis-je récupérer une VM supprimée depuis PVMSS ?**  
  La suppression d'une VM dans PVMSS est définitive du point de vue de l'application. Une récupération n'est possible que si vos administrateurs ont configuré des sauvegardes Proxmox et peuvent restaurer la VM à partir d'une sauvegarde en dehors de PVMSS.

## PVMSS vs Proxmox VE : que faire où ?

PVMSS fournit une interface self‑service simplifiée au‑dessus de Proxmox VE. Le tableau ci‑dessous résume où réaliser les actions les plus courantes :

| Action | PVMSS | Interface Proxmox VE |
| --- | --- | --- |
| Créer une VM KVM/QEMU | Oui (création self‑service dans les limites définies par les administrateurs) | Oui (toutes les options de configuration) |
| Créer un conteneur LXC | Non | Oui |
| Modifier les ressources de base d'une VM (CPU, RAM, nombre/taille de disques, cartes réseau, ISO) | Oui (dans les limites de l'UI et de la politique ; certaines opérations disque ne sont pas exposées) | Oui (ensemble complet d'options) |
| Gérer les snapshots | Non | Oui |
| Lancer des sauvegardes / restaurations | Non | Oui |
| Migrer des VMs entre nœuds (migration à chaud) | Non | Oui |
| Configurer la mise en réseau avancée (VLANs, règles de pare‑feu, etc.) | Partiellement (choix du pont et du modèle de carte uniquement) | Oui (pile réseau complète) |
| Gérer des templates de VM / clonage | Non | Oui |
| Configurer cloud-init | Non | Oui |
| Gérer les utilisateurs et permissions | Indirectement (votre compte et votre pool sont gérés par les administrateurs) | Oui (RBAC complet, royaumes, rôles, ACLs) |

Si vous avez besoin d'une fonctionnalité disponible uniquement dans Proxmox VE, contactez vos administrateurs pour qu'ils réalisent l'opération directement ou ajustent votre environnement.

## Limites connues

L'application PVMSS ne prend actuellement pas en charge :

- **Reconfiguration complète des ressources** : Même si vous pouvez modifier le CPU, la mémoire, les cartes réseau et l'ISO d'une VM arrêtée, certaines opérations restent indisponibles (par exemple l'agrandissement des disques, le changement du nombre de disques au-delà des limites définies par l'administrateur, ou la modification de certains paramètres bas-niveau Proxmox).
- **Conteneurs LXC** : Seules les machines virtuelles KVM/QEMU sont prises en charge. La création de conteneurs LXC n'est pas disponible.
- **Snapshots** : La création et la gestion de snapshots de VM ne sont pas disponibles via PVMSS.
- **Sauvegardes** : Les opérations de sauvegarde et de restauration de VM doivent être effectuées par les administrateurs directement via Proxmox.
- **Migration en direct** : Le déplacement de VMs entre nœuds n'est pas disponible via PVMSS.
- **Mise en réseau avancée** : Les fonctionnalités de mise en réseau avancées (VLANs, règles de pare-feu, etc.) doivent être configurées par les administrateurs, même si PVMSS supporte plusieurs cartes réseau et plusieurs modèles de cartes.
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
