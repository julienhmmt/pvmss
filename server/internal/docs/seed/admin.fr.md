# Guide de l'administration

Cette page est réservée aux administrateurs. Elle documente la surface d'administration et les tâches opérationnelles qui maintiennent PVMSS en bonne santé.

## Catalogue

La section **Catalogue** de la navigation administrateur permet d'approuver ou de masquer les nœuds, stockages, ponts, ISO, profils, tags et modèles cloud-init que la création de VM peut référencer. Les ressources découvertes apparaissent ici automatiquement ; basculez l'interrupteur pour contrôler ce que les utilisateurs voient.

## Politique

**Limites** définit les quotas par utilisateur (nombre maximal de VMs, CPU, mémoire, disque). **Capacité des nœuds** plafonne la part des ressources d'un nœud qu'une seule VM peut consommer. Les deux sont appliquées côté serveur avant tout appel Proxmox.

## Documentation

Cette page elle-même est gérée sous **Documentation** dans la navigation administrateur. Les administrateurs peuvent créer, modifier, activer, désactiver et supprimer des pages Markdown. Les pages intégrées (comme celle-ci) sont marquées **système** et ne peuvent pas être supprimées, mais leur contenu peut être édité. Chaque page a un public `user` (public) ou `admin` (réservé aux administrateurs).

## Système

La page **Informations sur l'application** affiche la version en cours et la configuration. La page **Paramètres** expose les interrupteurs opérationnels. Utilisez le journal d'audit pour retracer chaque écriture de VM jusqu'à l'utilisateur responsable.
