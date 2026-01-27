import json
import csv
from dataclasses import dataclass, field
from typing import List, Dict, Any, Optional, Callable
from datetime import datetime
from pathlib import Path


@dataclass
class AuthorStat:
    author: str
    count: int


@dataclass
class FileStat:
    file: str
    count: int


@dataclass
class TypeStat:
    type: str
    count: int


@dataclass
class ValueEntry:
    value: str
    masked_value: str
    occurrences: int
    authors: List[str]
    first_seen: str
    last_seen: str


@dataclass
class Secret:
    file: str
    key: str
    type: str
    change_count: int
    total_occurrences: int
    authors: List[str]
    first_seen: str
    last_seen: str
    history: List[ValueEntry]


@dataclass
class Stats:
    total_entries: int
    unique_secrets: int
    unique_values: int
    top_authors: List[AuthorStat]
    top_files: List[FileStat]
    type_breakdown: List[TypeStat]


@dataclass
class Analysis:
    stats: Stats
    secrets: List[Secret]


@dataclass
class StreamEntry:
    file: str
    key: str
    value: str
    masked_value: str
    type: str
    commit: str
    author: str
    date: str


@dataclass
class AnalyzeOptions:
    show_values: bool = False
    max_secrets: int = 0
    on_progress: Optional[Callable[[int], None]] = None


class Analyzer:
    def analyze_json(self, input_path: str, opts: AnalyzeOptions) -> Analysis:
        """Read JSON scan result, build stats and secrets"""
        with open(input_path, 'r') as f:
            data = json.load(f)

        index: Dict[str, Dict[str, Any]] = {}
        total_entries = 0

        if isinstance(data, dict) and 'results' in data:
            results = data['results']
        else:
            results = data if isinstance(data, list) else []

        for entry in results:
            total_entries += 1
            file = entry.get('file', '')
            key = entry.get('key', '')
            value = entry.get('value', '')
            type_ = entry.get('type', '')
            author = entry.get('author', '')
            date = entry.get('date', '')

            secret_key = f"{file}|{key}"

            if secret_key not in index:
                index[secret_key] = {
                    'file': file,
                    'key': key,
                    'type': type_,
                    'values': {},
                    'authors': set(),
                    'dates': [],
                    'change_count': 0
                }

            if value not in index[secret_key]['values']:
                index[secret_key]['values'][value] = {
                    'occurrences': 0,
                    'authors': set(),
                    'dates': []
                }

            index[secret_key]['values'][value]['occurrences'] += 1
            index[secret_key]['values'][value]['authors'].add(author)
            index[secret_key]['values'][value]['dates'].append(date)

            index[secret_key]['authors'].add(author)
            index[secret_key]['dates'].append(date)
            index[secret_key]['change_count'] += 1

        stats = self._build_stats(index, total_entries)
        return self._build_analysis(index, stats)

    def analyze_jsonl(self, input_path: str, opts: AnalyzeOptions) -> Analysis:
        """Read JSONL line by line with progress callback"""
        index: Dict[str, Dict[str, Any]] = {}
        total_entries = 0
        line_count = 0

        with open(input_path, 'r') as f:
            for line in f:
                line = line.strip()
                if not line:
                    continue

                entry = json.loads(line)
                total_entries += 1
                line_count += 1

                file = entry.get('file', '')
                key = entry.get('key', '')
                value = entry.get('value', '')
                type_ = entry.get('type', '')
                author = entry.get('author', '')
                date = entry.get('date', '')

                secret_key = f"{file}|{key}"

                if secret_key not in index:
                    index[secret_key] = {
                        'file': file,
                        'key': key,
                        'type': type_,
                        'values': {},
                        'authors': set(),
                        'dates': [],
                        'change_count': 0
                    }

                if value not in index[secret_key]['values']:
                    index[secret_key]['values'][value] = {
                        'occurrences': 0,
                        'authors': set(),
                        'dates': []
                    }

                index[secret_key]['values'][value]['occurrences'] += 1
                index[secret_key]['values'][value]['authors'].add(author)
                index[secret_key]['values'][value]['dates'].append(date)

                index[secret_key]['authors'].add(author)
                index[secret_key]['dates'].append(date)
                index[secret_key]['change_count'] += 1

                if opts.on_progress and line_count % 1000 == 0:
                    opts.on_progress(line_count)

        stats = self._build_stats(index, total_entries)
        return self._build_analysis(index, stats)

    def _build_stats(self, index: Dict[str, Dict[str, Any]], total_entries: int) -> Stats:
        """Build statistics from index"""
        author_counts: Dict[str, int] = {}
        file_counts: Dict[str, int] = {}
        type_counts: Dict[str, int] = {}
        unique_values = 0

        for secret_data in index.values():
            file = secret_data['file']
            type_ = secret_data['type']

            file_counts[file] = file_counts.get(file, 0) + 1
            type_counts[type_] = type_counts.get(type_, 0) + 1

            for author in secret_data['authors']:
                author_counts[author] = author_counts.get(author, 0) + 1

            unique_values += len(secret_data['values'])

        top_authors = sorted(
            [AuthorStat(author, count) for author, count in author_counts.items()],
            key=lambda x: x.count,
            reverse=True
        )[:10]

        top_files = sorted(
            [FileStat(file, count) for file, count in file_counts.items()],
            key=lambda x: x.count,
            reverse=True
        )[:10]

        type_breakdown = sorted(
            [TypeStat(type_, count) for type_, count in type_counts.items()],
            key=lambda x: x.count,
            reverse=True
        )

        return Stats(
            total_entries=total_entries,
            unique_secrets=len(index),
            unique_values=unique_values,
            top_authors=top_authors,
            top_files=top_files,
            type_breakdown=type_breakdown
        )

    def _build_analysis(self, index: Dict[str, Dict[str, Any]], stats: Stats) -> Analysis:
        """Convert internal index to Analysis"""
        secrets = []

        for secret_data in index.values():
            file = secret_data['file']
            key = secret_data['key']
            type_ = secret_data['type']
            change_count = secret_data['change_count']
            authors = sorted(list(secret_data['authors']))
            dates = sorted(secret_data['dates'])

            first_seen = dates[0] if dates else ''
            last_seen = dates[-1] if dates else ''

            history = []
            for value, value_data in secret_data['values'].items():
                masked_value = mask_secret(value)
                value_dates = sorted(value_data['dates'])
                value_entry = ValueEntry(
                    value=value,
                    masked_value=masked_value,
                    occurrences=value_data['occurrences'],
                    authors=sorted(list(value_data['authors'])),
                    first_seen=value_dates[0] if value_dates else '',
                    last_seen=value_dates[-1] if value_dates else ''
                )
                history.append(value_entry)

            history.sort(key=lambda x: x.occurrences, reverse=True)

            total_occurrences = sum(v.occurrences for v in history)

            secret = Secret(
                file=file,
                key=key,
                type=type_,
                change_count=change_count,
                total_occurrences=total_occurrences,
                authors=authors,
                first_seen=first_seen,
                last_seen=last_seen,
                history=history
            )
            secrets.append(secret)

        secrets.sort(key=lambda x: x.change_count, reverse=True)

        return Analysis(stats=stats, secrets=secrets)


def mask_secret(value: str) -> str:
    """Mask secret value: first2 + asterisks + last2"""
    if len(value) <= 4:
        return '****'
    return value[:2] + '*' * (len(value) - 4) + value[-2:]


def _compare_dates(a: str, b: str) -> int:
    """Compare RFC3339 dates. Return -1 if a < b, 0 if a == b, 1 if a > b"""
    try:
        date_a = datetime.fromisoformat(a.replace('Z', '+00:00'))
        date_b = datetime.fromisoformat(b.replace('Z', '+00:00'))

        if date_a < date_b:
            return -1
        elif date_a > date_b:
            return 1
        else:
            return 0
    except:
        return 0


def _escape_csv(s: str) -> str:
    """Escape string for CSV with semicolon delimiter"""
    if ';' in s or '\n' in s or '\r' in s or '"' in s:
        return '"' + s.replace('"', '""') + '"'
    return s


def _format_date(date_str: str) -> str:
    """Convert RFC3339 to YYYY-MM-DD"""
    try:
        dt = datetime.fromisoformat(date_str.replace('Z', '+00:00'))
        return dt.strftime('%Y-%m-%d')
    except:
        return date_str


def _truncate(s: str, max_len: int) -> str:
    """Truncate string with ellipsis"""
    if len(s) <= max_len:
        return s
    return s[:max_len-3] + '...'


def generate_report(analysis: Analysis, show_values: bool = False, max_secrets: int = 0) -> str:
    """Generate French-formatted text report"""
    lines = []

    lines.append('=' * 60)
    lines.append('RAPPORT D\'ANALYSE DES SECRETS')
    lines.append('=' * 60)
    lines.append('')

    lines.append('STATISTIQUES GLOBALES')
    lines.append('-' * 60)
    lines.append(f'Entrées totales: {analysis.stats.total_entries}')
    lines.append(f'Secrets uniques: {analysis.stats.unique_secrets}')
    lines.append(f'Valeurs uniques: {analysis.stats.unique_values}')
    lines.append('')

    if analysis.stats.top_authors:
        lines.append('TOP AUTEURS')
        lines.append('-' * 60)
        max_count = max(a.count for a in analysis.stats.top_authors) if analysis.stats.top_authors else 1
        for author_stat in analysis.stats.top_authors:
            bar_width = int((author_stat.count / max_count) * 20) if max_count > 0 else 0
            bar = '█' * bar_width
            lines.append(f'{author_stat.author:<30} {bar} {author_stat.count}')
        lines.append('')

    if analysis.stats.top_files:
        lines.append('TOP FICHIERS')
        lines.append('-' * 60)
        for file_stat in analysis.stats.top_files:
            lines.append(f'{_truncate(file_stat.file, 45):<50} {file_stat.count}')
        lines.append('')

    if analysis.stats.type_breakdown:
        lines.append('TYPES DE SECRETS')
        lines.append('-' * 60)
        for type_stat in analysis.stats.type_breakdown:
            lines.append(f'{type_stat.type:<50} {type_stat.count}')
        lines.append('')

    lines.append('SECRETS TRIÉS PAR FRÉQUENCE DE CHANGEMENT')
    lines.append('-' * 60)

    secrets_to_show = analysis.secrets
    if max_secrets > 0:
        secrets_to_show = secrets_to_show[:max_secrets]

    for i, secret in enumerate(secrets_to_show, 1):
        lines.append('')
        lines.append('┌' + '─' * 58 + '┐')
        lines.append(f'│ {i}. {_truncate(secret.key, 54):<55} │')
        lines.append(f'│ Fichier: {_truncate(secret.file, 49):<50} │')
        lines.append(f'│ Type: {secret.type:<53} │')
        lines.append(f'│ Changements: {secret.change_count:<44} │')
        lines.append(f'│ Total occurrences: {secret.total_occurrences:<39} │')
        lines.append(f'│ Auteurs: {", ".join(secret.authors):<47} │')
        lines.append(f'│ Premier vu: {_format_date(secret.first_seen):<45} │')
        lines.append(f'│ Dernier vu: {_format_date(secret.last_seen):<45} │')

        if show_values and secret.history:
            lines.append('│ Historique des valeurs:                            │')
            for value_entry in secret.history:
                masked = value_entry.masked_value
                if show_values:
                    masked = f'{value_entry.value[:10]}...' if len(value_entry.value) > 10 else value_entry.value
                lines.append(f'│   - {masked:<53} │')
                lines.append(f'│     Occurrences: {value_entry.occurrences}, Auteurs: {len(value_entry.authors):<31} │')

        lines.append('└' + '─' * 58 + '┘')

    lines.append('')

    return '\n'.join(lines)


def export_csv(analysis: Analysis, output_path: str) -> None:
    """Export secrets to CSV with BOM"""
    with open(output_path, 'w', encoding='utf-8-sig', newline='') as f:
        writer = csv.writer(f, delimiter=';')

        writer.writerow([
            'File',
            'Key',
            'Type',
            'ChangeCount',
            'TotalOccurrences',
            'Authors',
            'AuthorCount',
            'FirstSeen',
            'LastSeen',
            'DaysActive',
            'Values'
        ])

        for secret in analysis.secrets:
            authors_str = ', '.join(secret.authors)
            author_count = len(secret.authors)
            first_seen = _format_date(secret.first_seen)
            last_seen = _format_date(secret.last_seen)

            try:
                dt_first = datetime.fromisoformat(secret.first_seen.replace('Z', '+00:00'))
                dt_last = datetime.fromisoformat(secret.last_seen.replace('Z', '+00:00'))
                days_active = (dt_last - dt_first).days
            except:
                days_active = 0

            values_str = '; '.join([v.masked_value for v in secret.history])

            writer.writerow([
                secret.file,
                secret.key,
                secret.type,
                secret.change_count,
                secret.total_occurrences,
                authors_str,
                author_count,
                first_seen,
                last_seen,
                days_active,
                values_str
            ])


def export_stats_csv(analysis: Analysis, output_path: str) -> None:
    """Export statistics summary to CSV"""
    with open(output_path, 'w', encoding='utf-8-sig', newline='') as f:
        writer = csv.writer(f, delimiter=';')

        writer.writerow(['SUMMARY'])
        writer.writerow(['Total Entries', analysis.stats.total_entries])
        writer.writerow(['Unique Secrets', analysis.stats.unique_secrets])
        writer.writerow(['Unique Values', analysis.stats.unique_values])
        writer.writerow([])

        writer.writerow(['AUTHORS'])
        writer.writerow(['Author', 'Count'])
        for author_stat in analysis.stats.top_authors:
            writer.writerow([author_stat.author, author_stat.count])
        writer.writerow([])

        writer.writerow(['FILES'])
        writer.writerow(['File', 'Count'])
        for file_stat in analysis.stats.top_files:
            writer.writerow([file_stat.file, file_stat.count])
        writer.writerow([])

        writer.writerow(['SECRET TYPES'])
        writer.writerow(['Type', 'Count'])
        for type_stat in analysis.stats.type_breakdown:
            writer.writerow([type_stat.type, type_stat.count])
