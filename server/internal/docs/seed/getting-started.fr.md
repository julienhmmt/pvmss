# Premiers pas

Bienvenue sur PVMSS, le portail en libre-service pour vos machines virtuelles
Proxmox. Ce guide couvre l'essentiel : se connecter, retrouver ses VM et en
créer une nouvelle.

## Se connecter

Choisissez votre cluster et utilisez vos identifiants Proxmox sur la
[page de connexion](/login). Si le cluster sélectionné est injoignable, le
formulaire l'indique - réessayez plus tard ou choisissez un autre cluster.

## Retrouver ses VM

Une fois connecté, vous arrivez sur la page **Mes VM** (la barre latérale
affiche votre quota et les tâches encore en cours). Elle liste toutes les
machines virtuelles de votre pool sur l'ensemble des clusters configurés. Utilisez la
recherche pour filtrer par nom, VMID ou tag, et le sélecteur de cluster pour
restreindre la vue à un seul cluster. Les filtres sont conservés dans l'URL :
une vue filtrée peut être mise en favori.

## Créer une VM

1. Ouvrez **Créer une VM** depuis la page **Mes VM**.
2. Choisissez une source : une ISO ou un template Proxmox.
3. Choisissez un profil matériel ou saisissez des valeurs personnalisées ; le
   nœud est choisi pour vous sauf en mode Détaillé.
4. Donnez-lui un nom et validez - le portail provisionne la VM et affiche la
   progression dans le tiroir des tâches.

## Se connecter à sa VM

PVMSS n'ajoute ni clé SSH ni mot de passe à votre VM :

- **Template** - le système embarque les accès que votre équipe a intégrés au
  template (une clé SSH ou un mot de passe). Demandez-les à votre
  administrateur si vous ne les avez pas. L'onglet **Connexion** de la VM
  affiche la commande SSH dès que la machine signale son adresse.
- **ISO** - vous installez le système vous-même depuis la console du
  navigateur et choisissez votre compte pendant l'installation.

## Déplacer une VM

Les utilisateurs ne peuvent pas déplacer une VM d'un nœud à un autre. Pour
déplacer une VM, demandez à votre administrateur PVMSS.

Pour aller plus loin, consultez les [règles de création de VM](/docs/vm-creation-guidelines)
et le [guide utilisateur](/docs/user-guide).
