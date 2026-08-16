# Premiers pas

Bienvenue dans PVMSS, le portail en libre-service pour vos machines virtuelles Proxmox.
Ce guide vous présente l'essentiel : la connexion, la recherche de vos VMs et la
création d'une nouvelle machine.

## Se connecter

Utilisez vos identifiants Proxmox sur la [page de connexion](/login). Si votre
administrateur a activé l'authentification unique, vous pouvez aussi choisir le
fournisseur OIDC de votre cluster depuis l'écran de connexion.

## Trouver vos VMs

Une fois authentifié, la page **Mes VMs** liste toutes les machines virtuelles
appartenant à votre pool, sur l'ensemble des clusters configurés. Utilisez la
barre de recherche pour filtrer par nom ou VMID, et le sélecteur de cluster pour
limiter l'affichage à un seul cluster.

## Créer une VM

1. Ouvrez **Créer une VM** depuis la page d'accueil ou la page VMs.
2. Choisissez un profil matériel ou saisissez des valeurs personnalisées.
3. Sélectionnez un nœud, un stockage et (éventuellement) une ISO ou un modèle
   cloud-init.
4. Validez — le portail provisionne la VM et affiche la progression dans le
   bac de notification de la barre supérieure.

Pour en savoir plus, consultez les [recommandations de création de VM](/docs/vm-creation-guidelines).
