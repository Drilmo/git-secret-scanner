# ENTERPRISE.md - Guide de Déploiement Entreprise

## Git Secret Scanner - Version Python

**Version:** 1.0
**Date:** Janvier 2026
**Langue:** Français

---

## 1. Introduction

### Pourquoi cette version Python ?

La version Python de Git Secret Scanner a été développée pour répondre aux contraintes spécifiques des environnements d'entreprise :

- **Pas de binaires** : Certaines organisations interdisent les exécutables compilés pour des raisons de sécurité et de conformité
- **Compatibilité maximale** : Python est présent sur pratiquement tous les systèmes d'exploitation (Linux, macOS, Windows)
- **Transparence du code** : Le code source Python permet une audit de sécurité interne et une vérification complète
- **Facilité de déploiement** : Aucune étape de compilation, aucune dépendance système complexe

### Zéro dépendances externes

Cette implémentation utilise **exclusivement** la bibliothèque standard de Python :
- `json` pour le traitement des configurations et des résultats
- `subprocess` pour l'exécution des commandes Git
- `re` pour les expressions régulières
- `argparse` pour l'interface en ligne de commande
- `csv` pour l'export des données

**Aucune installation de paquet externe n'est nécessaire** (`pip install`). Vous pouvez donc l'utiliser dans les environnements sans accès à internet ou avec des restrictions réseau strictes.

### Compatibilité

- **Python 3.8+** (recommandé : Python 3.10 ou plus récent)
- Fonctionne sur Linux, macOS, et Windows
- Compatible avec les systèmes CI/CD majeurs (Jenkins, GitLab CI, GitHub Actions, Azure Pipelines)

---

## 2. Prérequis

Avant de déployer Git Secret Scanner dans votre environnement, vérifiez les éléments suivants :

### 2.1 Python 3.8+

Vérifiez la version installée :
```bash
python3 --version
```

Si une version antérieure à 3.8 est installée, téléchargez et installez une version plus récente depuis [python.org](https://www.python.org).

### 2.2 Git

Vérifiez que Git est installé et accessible :
```bash
git --version
```

La version minimale recommandée est Git 2.20. Pour les opérations de nettoyage avancées, Git 2.34+ est nécessaire.

### 2.3 (Optionnel) git-filter-repo

Pour les opérations de nettoyage avec les meilleures performances, installez `git-filter-repo` :

```bash
# Sur Linux/macOS
pip3 install git-filter-repo

# Ou téléchargez manuellement depuis:
# https://github.com/newren/git-filter-repo/releases
```

**Important** : Même sans `git-filter-repo`, l'outil fonctionne correctement avec `git filter-branch` natif.

### 2.4 Accès au dépôt Git

Assurez-vous d'avoir :
- Un accès en lecture à tous les dépôts à scanner
- Un accès en écriture si vous prévoyez d'utiliser la fonction de nettoyage (cleaning)

---

## 3. Méthodes d'installation

### Méthode A : Clonage Git (Recommandée)

Cette méthode est idéale pour les environnements ayant accès à Git et à internet.

```bash
# Clonez le dépôt
git clone https://github.com/votre-organisation/passwordAndSecretRemover.git

# Accédez au répertoire Python
cd passwordAndSecretRemover/python

# Lancez l'outil
python3 gitsecret.py
```

**Avantages :**
- Accès aux dernières mises à jour
- Facilité de mise à jour (`git pull`)
- Contrôle de version intégré

**Inconvénients :**
- Nécessite un accès réseau et un accès Git

### Méthode B : Copie des sources (Environnements restreints)

Pour les environnements sans accès internet ou avec restrictions réseau strict :

```bash
# Sur une machine avec accès internet :
git clone https://github.com/votre-organisation/passwordAndSecretRemover.git

# Compressez le répertoire python/
tar czf gitsecret-python.tar.gz passwordAndSecretRemover/python/

# Transférez gitsecret-python.tar.gz vers votre serveur cible
# (via USB, protocole interne, etc.)

# Sur la machine cible :
tar xzf gitsecret-python.tar.gz
cd python/
python3 gitsecret.py
```

**Avantages :**
- Aucune dépendance réseau après extraction
- Parfait pour les environnements air-gapped
- Aucune compilation nécessaire

**Inconvénients :**
- Nécessite un transfert manuel initial
- Plus complexe pour les mises à jour

### Méthode C : Archive tarball

Si vous avez les fichiers d'archive préparés :

```bash
# Récupérez gitsecret-python-v1.0.tar.gz

# Extrayez l'archive
tar xzf gitsecret-python-v1.0.tar.gz

# Accédez au répertoire
cd gitsecret-python/

# Lancez l'outil
python3 gitsecret.py
```

**Avantages :**
- Distribution standardisée et versionnable
- Taille de fichier réduite
- Facilité de versioning

### Méthode D : Exécution en tant que module

Une fois les sources en place :

```bash
cd /chemin/vers/python/

# Exécutez comme un module Python
python3 -m gitsecret
```

Cette méthode fonctionne aussi bien pour usage interactif que batch.

---

## 4. Configuration en entreprise

### 4.1 Patterns par défaut

Git Secret Scanner inclut des patterns de détection pour les types de secrets courants :
- Clés AWS, Azure, Google Cloud
- Tokens GitHub, GitLab, Bitbucket
- Clés d'API privées
- Mots de passe en clair
- Paramètres de connexion à base de données
- Certificats SSL/TLS

**Aucun fichier de configuration n'est nécessaire pour démarrer.** Les patterns par défaut sont intégrés au code.

### 4.2 Patterns personnalisés

Pour ajouter des patterns spécifiques à votre organisation, créez un fichier `patterns.json` :

```json
{
  "keyword_groups": {
    "company_api": {
      "keywords": ["ACME_API_KEY", "ACME_SECRET_TOKEN"],
      "regex": "^ACME_[A-Z0-9]{32}$",
      "description": "Clé API interne ACME"
    },
    "internal_db": {
      "keywords": ["DB_PASSWD", "DB_PASSWORD", "INTERNAL_PASSWORD"],
      "regex": "^(?=.*[a-z])(?=.*[A-Z])(?=.*\\d).{12,}$",
      "description": "Mot de passe base de données interne"
    }
  },
  "ignored_values": [
    "placeholder",
    "example",
    "todo"
  ],
  "ignored_files": [
    "*.lock",
    "package-lock.json"
  ]
}
```

### 4.3 Emplacements de configuration

L'outil recherche les fichiers de configuration dans cet ordre :
1. `./patterns.json` (répertoire courant)
2. `~/.config/git-secret-scanner/patterns.json` (répertoire home)
3. `/etc/git-secret-scanner/patterns.json` (configuration système - Linux/macOS)
4. Patterns par défaut intégrés

**Premier trouvé gagne** : une fois qu'un fichier de configuration est trouvé et valide, les suivants ne sont pas vérifiés.

### 4.4 Exemple : Ajouter des mots-clés spécifiques à l'organisation

Pour scanner les secrets spécifiques à votre entreprise, créez `patterns.json` dans le répertoire du dépôt :

```json
{
  "keyword_groups": {
    "company_secrets": {
      "keywords": [
        "CORPORATE_TOKEN",
        "ENTERPRISE_SECRET",
        "COMPANY_APIKEY"
      ],
      "regex": "[A-Z0-9]{40,}",
      "description": "Secrets propriétaires de l'entreprise"
    }
  }
}
```

Lancez ensuite l'outil : il utilisera automatiquement ces patterns en plus des patterns par défaut.

---

## 5. Workflows d'utilisation

### 5.1 Scan rapide

Pour scanner rapidement un dépôt et identifier les secrets :

```bash
# Accédez au dépôt à scanner
cd /chemin/vers/votre-repo

# Lancez le scan
python3 /chemin/vers/gitsecret.py scan

# Les résultats sont affichés à l'écran
# Un fichier scan_results.json est créé dans le répertoire courant
```

### 5.2 Workflow complet : Scan → Analyse → Nettoyage

#### Étape 1 : Scanner le dépôt

```bash
cd /chemin/vers/votre-repo
python3 /chemin/vers/gitsecret.py scan --mode full --output scan_results.json
```

**Options :**
- `--mode full` : Scan complet (résultats agrégés en JSON)
- `--mode stream` : Mode streaming (JSONL, pour grands dépôts)
- `--mode fast` : Scan rapide (moins de précision, plus rapide)
- `--output <fichier>` : Nom du fichier de sortie

#### Étape 2 : Analyser les résultats

```bash
python3 /chemin/vers/gitsecret.py analyze --input scan_results.json --output analysis.csv
```

Cela génère un fichier CSV avec :
- Top 10 des auteurs ayant commis des secrets
- Top 10 des fichiers contenant des secrets
- Répartition par type de secret
- Statistiques globales

#### Étape 3 : Examiner et approuver le nettoyage

```bash
# Consultez les résultats
cat analysis.csv

# Passez en revue scan_results.json
less scan_results.json

# Vérifiez que les secrets détectés sont bien des secrets réels
```

#### Étape 4 : Nettoyer les secrets

```bash
python3 /chemin/vers/gitsecret.py clean \
  --input scan_results.json \
  --backend git-filter-repo \
  --replacement "***REMOVED***"
```

**Options de nettoyage :**
- `--backend git-filter-repo` : Recommandé (plus rapide, plus fiable)
- `--backend bfg` : Alternative (nécessite BFG)
- `--backend git-filter-branch` : Fallback (natif Git, plus lent)
- `--replacement <texte>` : Texte de remplacement des secrets (défaut: `***REMOVED***`)

### 5.3 Formats de sortie

#### Format JSON (Scan complet)

```json
{
  "timestamp": "2026-01-27T14:30:00Z",
  "repository": "/path/to/repo",
  "scan_mode": "full",
  "total_commits": 523,
  "secrets_found": 47,
  "secrets": [
    {
      "type": "aws_access_key",
      "value": "AKIA2ABCD...",
      "file": "src/config.py",
      "commit": "abc1234",
      "author": "user@example.com",
      "line": 42
    }
  ]
}
```

#### Format JSONL (Mode streaming)

Chaque ligne est un objet JSON complet :
```
{"type": "aws_access_key", "value": "AKIA2...", "file": "src/config.py", ...}
{"type": "github_token", "value": "ghp_...", "file": "script.sh", ...}
```

Parfait pour les grands dépôts (traitement ligne par ligne).

#### Format CSV (Analyse)

```csv
Type,Nombre,Top Fichier,Top Auteur
AWS Access Key,12,src/config.py,john.doe@example.com
GitHub Token,8,deploy.sh,jane.smith@example.com
Database Password,15,secrets.env,admin@example.com
```

---

## 6. Intégration CI/CD

### 6.1 GitHub Actions

Créez `.github/workflows/secret-scan.yml` :

```yaml
name: Secret Scan

on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0  # Récupérez l'historique complet

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.10'

      - name: Download Git Secret Scanner
        run: |
          git clone https://github.com/votre-org/passwordAndSecretRemover.git
          cd passwordAndSecretRemover/python

      - name: Scan for secrets
        run: |
          python3 gitsecret.py scan --output secrets.json

      - name: Check for secrets
        run: |
          python3 gitsecret.py check --input secrets.json --fail-if-found
```

### 6.2 GitLab CI

Créez `.gitlab-ci.yml` :

```yaml
secret-scan:
  image: python:3.10
  script:
    - apt-get update && apt-get install -y git
    - git clone https://github.com/votre-org/passwordAndSecretRemover.git
    - cd passwordAndSecretRemover/python
    - python3 gitsecret.py scan --output secrets.json
    - python3 gitsecret.py check --input secrets.json --fail-if-found
  artifacts:
    paths:
      - secrets.json
    expire_in: 1 week
```

### 6.3 Jenkins

Créez un pipeline Jenkinsfile :

```groovy
pipeline {
  agent any

  stages {
    stage('Checkout') {
      steps {
        checkout scm
        sh 'git clone https://github.com/votre-org/passwordAndSecretRemover.git scanner'
      }
    }

    stage('Scan') {
      steps {
        sh '''
          cd scanner/python
          python3 gitsecret.py scan --output ${WORKSPACE}/secrets.json
        '''
      }
    }

    stage('Check') {
      steps {
        sh '''
          cd scanner/python
          python3 gitsecret.py check --input ${WORKSPACE}/secrets.json --fail-if-found
        '''
      }
    }
  }

  post {
    always {
      archiveArtifacts artifacts: 'secrets.json', allowEmptyArchive: true
    }
  }
}
```

### 6.4 Codes de sortie

L'outil retourne les codes de sortie suivants :

- **0** : Succès (aucun secret trouvé ou opération réussie)
- **1** : Erreur générale (fichier non trouvé, permissions, etc.)
- **2** : Secrets détectés (en mode scan avec `--fail-if-found`)
- **3** : Configuration invalide (fichier patterns.json mal formé)

Utilisez ces codes dans vos scripts :

```bash
python3 gitsecret.py scan --fail-if-found
if [ $? -eq 2 ]; then
  echo "Des secrets ont été détectés!"
  exit 1
fi
```

---

## 7. Considérations de sécurité

### 7.1 Environnement d'exécution isolé

Lancez Git Secret Scanner dans un environnement isolé pour minimiser les risques :

```bash
# Créez un utilisateur dédié (sur les serveurs)
sudo useradd -r -s /bin/bash gitsecret

# Donnez-lui accès uniquement aux dépôts à scanner
# Évitez les accès administrateur
```

### 7.2 Les fichiers de résultats contiennent des données sensibles

**ATTENTION** : Les fichiers `scan_results.json` contiennent les secrets détectés en clair !

Protégez-les strictement :

```bash
# Définissez les permissions restrictives
chmod 600 scan_results.json
chmod 600 analysis.csv

# Chiffrez les fichiers sensibles
gpg --encrypt --recipient security-team@example.com scan_results.json

# Stockez dans un endroit sécurisé (crypté, accès limité)
# N'engagez JAMAIS ces fichiers dans Git !
```

### 7.3 Gestion des résultats de scan

Pour chaque scan :

1. **Examinez les résultats** avec une personne autorisée (responsable sécurité)
2. **Approuvez le nettoyage** avant d'effectuer les modifications d'historique
3. **Archivez les résultats** de manière sécurisée (chiffré, audit trail)
4. **Supprimez les fichiers temporaires** après utilisation :
   ```bash
   shred -uvfz scan_results.json analysis.csv
   ```

### 7.4 Rotation des secrets après nettoyage

Après un nettoyage d'historique, les secrets restent potentiellement actifs (dans la mémoire de Git, caches, sauvegardes, etc.) :

1. **Régénérez tous les secrets détectés** (nouvelles clés API, tokens, passwords)
2. **Invalidez les anciens secrets** immédiatement
3. **Configurez la rotation automatique** pour les futures détections :
   ```bash
   # Exemple avec AWS CLI
   aws iam delete-access-key --access-key-id AKIA2ABCD...
   ```

### 7.5 Audit et logging

Enregistrez toutes les opérations de scan et nettoyage :

```bash
# Définissez une variable d'environnement pour le logging
export GITSECRET_LOG_LEVEL=DEBUG
export GITSECRET_LOG_FILE=/var/log/gitsecret/operations.log

python3 gitsecret.py scan ...
```

Vérifiez régulièrement les journaux pour toute activité suspecte.

---

## 8. Dépannage

### 8.1 Problèmes de version Python

**Erreur :** `python3: command not found`

```bash
# Sur Debian/Ubuntu
sudo apt-get install python3.10

# Sur RHEL/CentOS
sudo yum install python3.10

# Sur macOS
brew install python@3.10
```

**Erreur :** `SyntaxError: invalid syntax` (Python 3.7 ou antérieur)

```bash
# Vérifiez votre version
python3 --version

# Installez Python 3.8+
# Puis utilisez explicitement la bonne version
python3.10 gitsecret.py scan
```

### 8.2 Installation de git-filter-repo derrière un proxy

Si `pip install` échoue derrière un proxy corporatif :

```bash
# Définissez les paramètres du proxy
pip install --proxy [user:passwd@]proxy.server:port git-filter-repo

# Ou créez un fichier de configuration ~/.pip/pip.conf
[global]
proxy = [user:passwd@]proxy.server:port

# Puis réessayez
pip install git-filter-repo
```

### 8.3 Gestion de grands dépôts

Pour les dépôts avec des milliers de commits :

```bash
# Utilisez le mode streaming (plus rapide, moins de mémoire)
python3 gitsecret.py scan --mode stream --output secrets.jsonl

# Pour l'analyse, les résultats JSONL sont automatiquement gérés
python3 gitsecret.py analyze --input secrets.jsonl
```

Si le scan s'arrête ou est très lent :

```bash
# Limitez la plage de commits scannés
python3 gitsecret.py scan \
  --since "2 years ago" \
  --until "1 year ago" \
  --output secrets_range.json
```

### 8.4 Messages d'erreur courants

| Erreur | Cause | Solution |
|--------|-------|----------|
| `fatal: not a git repository` | Le répertoire courant n'est pas un dépôt Git | Naviguez vers un dépôt Git valide |
| `Permission denied: 'git'` | Git n'est pas installé ou pas accessible | Installez Git ou vérifiez le PATH |
| `Invalid JSON in patterns.json` | Fichier de configuration malformé | Validez le JSON avec `python3 -m json.tool patterns.json` |
| `No secrets found` | Aucun secret détecté (attendu) | Normal si le dépôt est sain |
| `FileNotFoundError: scan_results.json` | Le fichier de sortie du scan n'existe pas | Lancez d'abord le scan (`scan`) avant l'analyse |

### 8.5 Validation de configuration JSON

Validez vos fichiers patterns.json avant utilisation :

```bash
python3 -m json.tool patterns.json > /dev/null && echo "Valid JSON" || echo "Invalid JSON"
```

---

## 9. Aperçu de l'architecture

### 9.1 Structure des modules

```
python/
├── gitsecret.py              # Point d'entrée principal
├── modules/
│   ├── scanner.py            # Module de scan d'historique Git
│   ├── analyzer.py           # Module d'analyse des résultats
│   ├── cleaner.py            # Module de nettoyage d'historique
│   ├── config.py             # Module de gestion configuration
│   └── utils.py              # Utilitaires (logging, formatage)
├── patterns/
│   └── default_patterns.json # Patterns de détection par défaut
└── tests/
    └── test_*.py             # Suite de tests
```

### 9.2 Flux de données

```
┌─────────────────────────────────────────────────────────────┐
│ 1. SCAN                                                     │
│ ───────────────────────────────────────────────────────────│
│ Utilisateur lance gitsecret.py scan                        │
│        ↓                                                    │
│ Config charge patterns (local/home/built-in)              │
│        ↓                                                    │
│ Scanner exécute: git log --all -S <keyword>              │
│        ↓                                                    │
│ Extraction: applique regex sur résultats                 │
│        ↓                                                    │
│ Sortie: JSON/JSONL avec secrets détectés                 │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. ANALYZE                                                  │
│ ───────────────────────────────────────────────────────────│
│ Utilisateur lance gitsecret.py analyze                    │
│        ↓                                                    │
│ Analyzer lit JSON/JSONL                                    │
│        ↓                                                    │
│ Calcule statistiques:                                      │
│   - Top 10 auteurs                                        │
│   - Top 10 fichiers                                       │
│   - Répartition par type                                  │
│        ↓                                                    │
│ Export CSV pour rapport                                    │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. CLEAN                                                    │
│ ───────────────────────────────────────────────────────────│
│ Utilisateur lance gitsecret.py clean                      │
│        ↓                                                    │
│ Cleaner lit résultats scan                                │
│        ↓                                                    │
│ Sélectionne backend (git-filter-repo/bfg/git-filter-br) │
│        ↓                                                    │
│ Réécrit historique: secrets → ***REMOVED***              │
│        ↓                                                    │
│ Rebase/force push (avec approbation)                      │
└─────────────────────────────────────────────────────────────┘
```

### 9.3 Format de sortie compatible

La version Python produit des résultats identiques à la version Go :

```json
{
  "timestamp": "2026-01-27T14:30:00Z",
  "repository": "/path/to/repo",
  "scan_mode": "full",
  "secrets": [
    {
      "type": "aws_access_key",
      "value": "AKIA...",
      "file": "config.py",
      "commit": "abc123",
      "author": "user@example.com"
    }
  ]
}
```

Vous pouvez migrer les résultats entre versions Python et Go sans problème.

### 9.4 Diagramme des dépendances

```
gitsecret.py
├── argparse (Python stdlib)
├── json (Python stdlib)
├── subprocess → git (externe)
├── re (Python stdlib)
├── modules/
│   ├── scanner.py
│   │   └── subprocess.run(['git', 'log', ...])
│   ├── analyzer.py
│   │   └── json, csv (stdlib)
│   ├── cleaner.py
│   │   └── subprocess.run(['git-filter-repo'/'bfg'/'git'])
│   ├── config.py
│   │   └── json (stdlib)
│   └── utils.py
│       └── logging (stdlib)
```

**Total des dépendances externes :** 1 (Git - déjà installé)

---

## 10. Support et maintenance

### 10.1 Mise à jour

Pour mettre à jour la version Python :

```bash
# Si installé via Git
cd /chemin/vers/passwordAndSecretRemover
git pull origin main

# Si installé via archive
# Téléchargez la nouvelle version et comparez les fichiers
```

### 10.2 Signaler des problèmes

Pour signaler un bug ou une demande d'amélioration :

```bash
# Exemple de rapport de bug
# Titre : "Erreur: Python 3.8 sur RHEL 7"
# Description :
# - Système : RHEL 7
# - Version Python : 3.8.5
# - Erreur : ImportError: no module named 'subprocess'
# - Commande : python3 gitsecret.py scan
# - Trace d'erreur : [coller la sortie complète]
```

### 10.3 Contribution et sécurité

Pour les contributions ou découvertes de vulnérabilités :

- **Contributions** : Soumettez une pull request sur le dépôt principal
- **Vulnérabilités** : Contactez l'équipe sécurité (security@example.com) de manière confidentielle

---

## 11. Checklist de déploiement

Avant de déployer en production, vérifiez :

- [ ] Python 3.8+ installé et testé
- [ ] Git installé et accessible
- [ ] Sources téléchargées ou transférées avec succès
- [ ] Permissions d'accès vérifiées (utilisateur dédié si possible)
- [ ] Fichier patterns.json personnalisé créé (si nécessaire)
- [ ] Test du scan sur un petit dépôt
- [ ] Résultats examinés et validés
- [ ] Procédure de nettoyage testée en environnement non-production
- [ ] Logging/audit activé
- [ ] Documentation interne mise à jour
- [ ] Plan de rotation des secrets en place
- [ ] Formation de l'équipe complétée

---

## 12. Ressources supplémentaires

### Documentation connexe
- Git Documentation: https://git-scm.com/doc
- Python 3 Standard Library: https://docs.python.org/3/library/
- git-filter-repo: https://github.com/newren/git-filter-repo

### Outils complémentaires
- **git-secrets** : Prévention en amont (hook local)
- **TruffleHog** : Détection supplémentaire
- **OWASP Secret Detection** : Références de patterns OWASP

### Support interne
- **Équipe Sécurité** : security@example.com
- **Équipe Infrastructure** : infrastructure@example.com
- **Équipe DevOps** : devops@example.com

---

**Document de contrôle :**
- Auteur : Équipe Sécurité
- Version : 1.0
- Date de dernière révision : Janvier 2026
- Révision suivante prévue : Janvier 2027

---

*Ce document est fourni à titre informatif. Les procédures doivent être adaptées à votre environnement spécifique. Consultez votre équipe sécurité avant de déployer en production.*
