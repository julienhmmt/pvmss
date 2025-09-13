# Guide de l'utilisateur de PVMSS

PVMSS (Proxmox Virtual Machine Self-Service) est une application web intuitive permettant de créer, gérer et accéder aux consoles des machines virtuelles de manière simplifiée.

L'application est disponible en français et en anglais.

Pour accéder aux fonctionnalités principales, telles que la création de machines virtuelles et l'accès à leur console, vous devez vous connecter à l'aide des identifiants fournis par l'administrateur de votre organisation.

## Guide de démarrage rapide

1. **Connexion à l'application** : Connectez-vous à PVMSS à l'aide de vos identifiants pour accéder aux fonctionnalités de création et de gestion des machines virtuelles.
2. **Recherche de machines virtuelles** : Utilisez la fonction de recherche pour localiser une machine virtuelle spécifique et consulter ses détails.
3. **Création d'une machine virtuelle** : Cliquez sur le bouton "Créer une machine virtuelle" pour ouvrir le formulaire de configuration, puis renseignez les paramètres requis.
4. **Accès à la console** : Une fois la machine virtuelle créée, cliquez sur le bouton "Console" pour vous connecter à son interface.

## Fonctionnalités principales

### Création d'une machine virtuelle

Pour créer une machine virtuelle, accédez au formulaire de configuration via le bouton "Créer une VM" après vous être connecté à PVMSS. Les paramètres suivants doivent être définis :

- **Nom et description** : Saisissez un nom unique et une description pour identifier votre machine virtuelle.
- **Système d'exploitation** : Sélectionnez une image ISO parmi une liste prédéfinie par les administrateurs pour installer le système d'exploitation.
- **Ressources** : Configurez les ressources nécessaires, notamment le nombre de cœurs CPU, la mémoire vive, le stockage et le pont réseau.
- **Tags** : Ajoutez des tags prédéfinis pour organiser et faciliter la recherche de vos machines virtuelles.

Vous ne pouvez créer qu'une seule machine à la fois, et vous ne pouvez pas modifier les ressources d'une machine virtuelle existante.

### Recherche d'une machine virtuelle

Utilisez la fonction de recherche pour localiser une machine virtuelle par son *nom* ou son *VMID* (identifiant unique attribué à chaque machine virtuelle). Une liste de résultats s'affichera en fonction des critères saisis. Cliquez sur le bouton "*Détails*" pour consulter les informations de la machine virtuelle et gérer son statut.

### Gestion d'une machine virtuelle

Un panneau intuitif permet de gérer vos machines virtuelles, d'accéder à leurs détails et de surveiller leur utilisation.

- **Contrôle** : Démarrez, arrêtez ou redémarrez la machine virtuelle.
- **Surveillance** : Consultez en temps réel l'état, le temps de fonctionnement et l'utilisation des ressources (CPU, mémoire, stockage).
- **Détails et modifications** : Accédez aux informations de configuration et modifiez la description ou les tags si nécessaire.

## Bonnes pratiques

- Arrêtez proprement vos machines virtuelles pour éviter toute perte de données.
- Utilisez des noms clairs et conformes aux normes de votre organisation pour vos machines virtuelles.
- Contactez votre administrateur pour toute demande de ressources supplémentaires.
- Ne partagez jamais vos identifiants de connexion afin de garantir la sécurité de votre compte et de vos machines virtuelles.

## Support

L'application PVMSS est gérée par l'équipe informatique de votre organisation. Pour toute assistance, contactez votre administrateur dans les cas suivants :

- Perte de votre mot de passe (la réinitialisation n'est pas disponible en libre-service).
- Difficultés lors de la création d'une machine virtuelle.
- Problèmes d'accès à la console.

## Limites connues

L'application PVMSS ne permet pas de :

- Modifier les ressources (CPU, mémoire, stockage, pont réseau) d'une machine virtuelle.
- Changer votre identifiant et mot de passe de connexion.
- Créer des conteneurs LXC.
