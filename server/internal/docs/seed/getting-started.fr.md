# Premiers pas

Bienvenue sur PVMSS, le portail en libre-service pour vos machines virtuelles
Proxmox. Ce guide couvre l'essentiel : se connecter, retrouver ses VM et en
créer une nouvelle.

## Se connecter

Choisissez votre cluster et utilisez vos identifiants Proxmox sur la
[page de connexion](/login). Si le cluster sélectionné est injoignable, le
formulaire l'indique - réessayez plus tard ou choisissez un autre cluster.

## Retrouver ses VM

Une fois connecté, la page d'accueil affiche le nombre de vos VM, votre quota
et les tâches encore en cours. La page **Mes VM** liste toutes les machines
virtuelles de votre pool sur l'ensemble des clusters configurés. Utilisez la
recherche pour filtrer par nom, VMID ou tag, et le sélecteur de cluster pour
restreindre la vue à un seul cluster. Les filtres sont conservés dans l'URL :
une vue filtrée peut être mise en favori.

## Créer une VM

1. Ouvrez **Créer une VM** depuis la page d'accueil ou la page des VM.
2. Choisissez une source : une ISO, un template Proxmox ou une image cloud.
3. Choisissez un profil matériel ou saisissez des valeurs personnalisées ; le
   nœud est choisi pour vous sauf en mode Détaillé.
4. Attachez éventuellement un document cloud-init (template admin ou l'un de
   [vos fichiers](/cloud-init)).
5. Validez - le portail provisionne la VM et affiche la progression dans le
   tiroir des tâches.

Pour aller plus loin, consultez les [règles de création de VM](/docs/vm-creation-guidelines)
et le [guide utilisateur](/docs/user-guide).
