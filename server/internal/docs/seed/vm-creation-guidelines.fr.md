# Recommandations de création de VM

Cette page rassemble les règles et recommandations qui encadrent la création de
VM dans PVMSS. Votre administrateur peut imposer des limites plus strictes via
une politique ; les valeurs indiquées ici sont les réglages par défaut du portail.

## Nommage

Les noms de VM sont en minuscules, séparés par des traits d'union et uniques au
sein de votre pool. Évitez les noms génériques comme `vm1` — un nom descriptif
(`web-prod-01`) rend la liste des VMs recherchable et le journal d'audit lisible.

## Ressources

- Partez d'un **profil** lorsqu'il correspond à votre charge de travail ; les profils encapsulent les combinaisons CPU, mémoire et disque approuvées et gardent le catalogue cohérent.
- Les valeurs personnalisées sont plafonnées par la politique du cluster : les demandes dépassant le quota par utilisateur ou la capacité d'un nœud sont rejetées avant tout appel à Proxmox.
- Les disques utilisent le stockage que vous sélectionnez ; choisissez un stockage adapté au profil d'E/S attendu du disque.

## Cloud-init

Préférez un **modèle cloud-init** à une configuration manuelle post-installation. Les modèles sont validés et curés par les administrateurs côté serveur ; en sélectionner un garantit que le snippet est bien formé. Consultez le [guide cloud-init](/docs/cloud-init-howto) pour les champs pris en charge.

## Après la création

Les nouvelles VMs apparaissent dans **Mes VMs** immédiatement. Le premier démarrage peut prendre une minute le temps que cloud-init s'exécute ; l'onglet console affiche la sortie de démarrage en direct.
