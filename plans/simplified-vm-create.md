## Plan de tâches techniques

### 1. Introduire le concept de profil de VM
Créer une notion de `VMProfile` ou `TemplateProfile` côté backend. Chaque profil doit contenir : `id`, `label`, `description`, `defaultCpu`, `defaultRamMb`, `defaultDiskGb`, et éventuellement des champs comme `networkModel`, `bootMode`, `tags`, `cloudInit`, `allowGpu`, `allowAdvancedOverrides`. Cette couche servira de base au mode simplifié et permettra de réutiliser la même logique pour toutes les créations.[3][4]

### 2. Définir les 6 profils standards
Ajouter 6 profils initiaux :
- Serveur web.
- API légère.
- Serveur applicatif.
- Base de test.
- Bastion.
- Poste technique.

Chacun doit avoir une configuration par défaut stable, par exemple 1 vCPU / 2 Go pour le serveur web, 2 vCPU / 2 Go pour une API légère, etc. Les profils doivent être stockés dans une config versionnée ou dans un fichier de seed, afin que l’administrateur puisse les ajuster sans changer toute la logique métier.[4][3]

### 3. Ajouter un mode de création « simplifié »
Créer un flux de création dédié avec uniquement :
- choix du profil,
- nom de la VM,
- ISO,
- écran de confirmation.

L’interface simplifiée ne doit pas exposer les réglages CPU/RAM/disque, sauf si un profil autorise une override ciblée. Le but est de transformer l’expérience en assistant guidé, avec des paramètres invisibles remplis par défaut à partir du profil choisi.[5][1]

### 4. Garder un mode « expert »
Conserver le formulaire complet actuel dans un écran ou un onglet séparé. Le mode expert doit permettre de modifier tous les paramètres disponibles aujourd’hui, y compris les options avancées de Proxmox. Les deux modes doivent appeler la même fonction de résolution de configuration finale, pour éviter les différences de comportement.[2][4]

### 5. Créer une fonction de résolution
Implémenter une fonction backend du type `ResolveVmCreationConfig(profile, request)`. Cette fonction doit :
- charger le profil,
- fusionner les valeurs par défaut,
- appliquer les champs saisis,
- vérifier les contraintes du mode,
- produire un objet final prêt pour l’API Proxmox.

Cette étape est centrale, car elle évite de dupliquer la logique entre simplifié et expert.[2][3]

### 6. Ajouter les règles de validation
Mettre en place des validations distinctes :
- validation commune : nom VM, ISO, profil.
- validation simplifiée : interdiction des paramètres avancés.
- validation expert : contrôle complet.
- validation de sécurité : refus des réglages non autorisés ou hors politique.

Prévoir aussi un test explicite pour les cas spéciaux comme GPU, VLAN spécifique, passthrough, stockage atypique ou options Proxmox avancées, car ces cas dépassent généralement un catalogue standard.[3][4]

### 7. Prévoir le message d’escalade administrateur
Ajouter un message visible dans l’UI simplifiée, par exemple :
> Les modèles proposés couvrent les usages courants. Pour un besoin particulier, comme un GPU, un réseau avancé ou un stockage spécifique, merci de contacter votre administrateur PVMSS.

Ce message doit apparaître avant la validation finale, ainsi que dans les erreurs de validation si l’utilisateur tente une option non autorisée.[1][4]

### 8. Adapter le frontend
Découper l’interface en composants :
- `ProfileCardList` pour les 6 modèles.
- `SimplifiedVmWizard` pour le parcours court.
- `ExpertVmForm` pour le formulaire complet.
- `SummaryPanel` pour le récapitulatif.
- `AdminNotice` pour le message d’escalade.

L’UX doit permettre de basculer du mode simplifié au mode expert sans perte de données. Cela rend le produit plus accessible tout en gardant les capacités avancées.[5][1]

### 9. Structurer l’API
Prévoir des endpoints clairs :
- `GET /vm-profiles`
- `POST /vms/create/simplified`
- `POST /vms/create/expert`
- ou un seul endpoint `POST /vms/create` avec `mode=simplified|expert`.

La logique serveur doit rester unique, même si les routes d’entrée diffèrent. Le simplifié peut envoyer seulement quelques champs, puis le serveur complète avec le profil.[2]

### 10. Ajouter les tests
Créer des tests unitaires pour :
- résolution des profils,
- validation simplifiée,
- validation expert,
- message d’escalade,
- compatibilité des paramètres Proxmox.

Ajouter des tests d’intégration sur la création de VM afin de vérifier que la configuration finale correspond bien au profil sélectionné. Les templates et clones Proxmox reposent sur des bases standardisées, donc ces tests sont essentiels pour éviter les régressions.[4][3]

## Prompt LLM

Voici le prompt que tu peux donner à un LLM pour générer le travail d’implémentation :

```markdown
Tu es un développeur senior Go et frontend.

Je travaille sur PVMSS, une application self-service pour créer des VM Proxmox.
Je veux ajouter deux modes de création :
- un mode simplifié,
- un mode expert.

Contexte produit :
- Le mode simplifié doit proposer 6 modèles types :
  - Serveur web
  - API légère
  - Serveur applicatif
  - Base de test
  - Bastion
  - Poste technique
- En mode simplifié, l’utilisateur ne doit saisir que :
  - le nom de la VM,
  - l’ISO,
  - le modèle.
- Les autres paramètres doivent être automatiquement préremplis à partir du modèle.
- Si l’utilisateur a un besoin particulier, par exemple GPU, réseau avancé, stockage spécifique ou options Proxmox non standard, il doit être invité à contacter l’administrateur PVMSS.

Objectif :
Produis un plan de mise en œuvre technique détaillé et actionnable pour intégrer ce système dans l’application.

Attendus :
1. Propose l’architecture générale.
2. Définis les structures Go à créer ou modifier.
3. Décris les composants frontend à créer ou refactorer.
4. Décris les endpoints API à ajouter ou adapter.
5. Décris la logique de validation commune et par mode.
6. Décris comment fusionner les valeurs du modèle avec les données utilisateur.
7. Propose un plan de tests unitaires et d’intégration.
8. Donne un plan de migration étape par étape.
9. Donne les messages UI à afficher à l’utilisateur, y compris le message d’escalade vers l’administrateur.

Contraintes :
- Ne pas dupliquer la logique de création VM.
- Le mode simplifié et le mode expert doivent partager la même couche métier.
- La solution doit être maintenable et facilement extensible.
- Réponds en markdown.
- Utilise des exemples concrets de structs Go et de noms de fichiers/composants.
- Sois précis et orienté implémentation.
- Si une règle métier doit bloquer une configuration, explique comment la gérer proprement.

Livrables :
- Un plan de tâches techniques détaillé.
- Les structures Go proposées.
- Les composants frontend.
- Les endpoints API.
- Les tests à écrire.
- Le texte UX d’avertissement administrateur.
```

