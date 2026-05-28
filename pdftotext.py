#!/usr/bin/env python3
"""
Конвертер PDF -> Markdown с помощью Xpdf pdftotext.

На вход подаётся папка с PDF-файлами. Скрипт находит все .pdf,
конвертирует каждый в .md с тем же именем, используя утилиту
pdftotext.exe из состава Xpdf Tools.

Поддерживает русский (кириллица) и английский языки через UTF-8.

Использование:
    python pdftotext.py <путь_к_папке>
    python pdftotext.py                     # конвертировать PDF в текущей папке
"""

import argparse
import concurrent.futures
import os
import subprocess
import sys
import tempfile
import threading
from pathlib import Path


# ========== ПУТИ К ИНСТРУМЕНТАМ (при необходимости отредактируйте) ==========

# Путь к папке с Xpdf Tools (версия 4.06)
XPDF_DIR = Path(r"C:\Users\e19700019\Downloads\xpdf-tools-win-4.06")

# Используем 64-битную версию pdftotext.exe
# Если у вас 32-битная система, замените bin64 на bin32
PDFTOTEXT_EXE = XPDF_DIR / "bin64" / "pdftotext.exe"

# Путь к файлам поддержки кириллицы (русские шрифты/кодировки)
CYRILLIC_DIR = Path(
    r"C:\Users\e19700019\Downloads\xpdf-cyrillic.tar\xpdf-cyrillic"
)

# Файл сопоставления имён символов с Unicode (для болгарской/русской кириллицы)
NAME_TO_UNICODE = CYRILLIC_DIR / "Bulgarian.nameToUnicode"

# Unicode-карта для кодировки KOI8-R (нужна для корректного вывода русского текста)
UNICODE_MAP = CYRILLIC_DIR / "KOI8-R.unicodeMap"


def check_prerequisites() -> None:
    """
    Проверяет, что все необходимые файлы существуют на диске.

    Если чего-то не хватает — выводит сообщения об ошибках и завершает
    скрипт с кодом 1, чтобы пользователь сразу понял, что нужно поправить.
    """
    errors = []

    # Проверяем, есть ли сам pdftotext.exe
    if not PDFTOTEXT_EXE.is_file():
        errors.append(
            f"pdftotext.exe не найден: {PDFTOTEXT_EXE}\n"
            f"Ожидался в: {XPDF_DIR / 'bin64' / 'pdftotext.exe'}"
        )

    # Проверяем папку с кириллическими файлами
    if not CYRILLIC_DIR.is_dir():
        errors.append(f"Папка кириллической поддержки не найдена: {CYRILLIC_DIR}")

    # Проверяем файл сопоставления имён символов
    if not NAME_TO_UNICODE.is_file():
        errors.append(f"Файл не найден: {NAME_TO_UNICODE}")

    # Проверяем файл Unicode-карты KOI8-R
    if not UNICODE_MAP.is_file():
        errors.append(f"Файл не найден: {UNICODE_MAP}")

    # Если есть ошибки — печатаем их и выходим
    if errors:
        print("ОШИБКА: не найдены необходимые файлы:", file=sys.stderr)
        for err in errors:
            print(f"  - {err}", file=sys.stderr)
        sys.exit(1)


def create_xpdfrc(xpdfrc_path: Path) -> None:
    """
    Создаёт временный конфигурационный файл xpdfrc.

    Зачем это нужно:
    - По умолчанию pdftotext не знает о кириллических шрифтах.
    - Мы подключаем Bulgarian.nameToUnicode — он помогает правильно
      распознавать имена символов в PDF (кириллица).
    - Подключаем KOI8-R.unicodeMap — карта для преобразования KOI8-R в Unicode.
    - Устанавливаем textEncoding=UTF-8, чтобы на выходе был читаемый
      текст в UTF-8 (поддерживает и русский, и английский).
    """
    content = f"""# xpdfrc — сгенерировано pdftotext.py
nameToUnicode\t\t{NAME_TO_UNICODE}
unicodeMap\tKOI8-R\t\t{UNICODE_MAP}
textEncoding\tUTF-8
"""
    xpdfrc_path.write_text(content, encoding="utf-8")


def convert_pdf(pdf_path: Path, output_path: Path, xpdfrc_path: Path) -> bool:
    """
    Конвертирует один PDF-файл в Markdown с помощью pdftotext.

    Параметры команды pdftotext:
      -cfg <файл>   — указываем наш конфиг с кириллицей и UTF-8
      -enc UTF-8    — выходная кодировка: Unicode (для русского + английского)
      -layout       — сохраняет физическую структуру страницы (колонки, таблицы)
      -nopgbrk      — не вставлять символы разрыва страницы (чтобы .md был чистым)
      -eol dos      — концы строк в формате Windows (CR+LF)

    Возвращает True при успехе, False при ошибке.
    """
    # Формируем команду для запуска pdftotext
    cmd = [
        str(PDFTOTEXT_EXE),
        "-cfg", str(xpdfrc_path),
        "-enc", "UTF-8",
        "-layout",
        "-nopgbrk",
        "-eol", "dos",
        str(pdf_path),
        str(output_path),
    ]

    try:
        # Запускаем pdftotext и ждём завершения
        result = subprocess.run(
            cmd,
            capture_output=True,  # перехватываем stdout/stderr
            text=True,
        )
    except FileNotFoundError:
        # Если pdftotext.exe вообще не найден (например, удалили после проверки)
        print(f"  ОШИБКА: pdftotext.exe не найден: {PDFTOTEXT_EXE}", file=sys.stderr)
        return False
    except OSError as e:
        # Любая другая системная ошибка при запуске
        print(f"  ОШИБКА: не удалось запустить pdftotext: {e}", file=sys.stderr)
        return False

    # Проверяем код возврата: 0 = успех, иначе ошибка
    if result.returncode != 0:
        stderr = result.stderr.strip()
        print(
            f"  ОШИБКА: pdftotext завершился с кодом {result.returncode}"
            + (f" ({stderr})" if stderr else ""),
            file=sys.stderr,
        )
        return False

    return True


def trim_output_file(output_path: Path) -> bool:
    """
    Обрезает лишние пробелы в начале и конце каждой строки файла.

    Затем, если после обработки файл оказался пустым (нет ни одного
    непустого символа) — удаляет его и возвращает False.

    Это нужно для двух целей:
      1. Убрать «мусорные» пробелы, которые pdftotext иногда оставляет.
      2. Не создавать .md файлы, если PDF оказался пустым или
         нечитаемым (pdftotext создаёт пустой файл, но он нам не нужен).
    """
    try:
        # Читаем исходный текст
        lines = output_path.read_text(encoding="utf-8").splitlines(keepends=True)
    except Exception:
        # Если файл не читается — не трогаем его
        return True

    # Обрезаем пробелы в начале и конце каждой строки
    trimmed_lines = [line.strip() + "\n" for line in lines]
    # Удаляем лишний символ новой строки в самом конце (чтобы не было двойного \n)
    # strip сам уберёт этот пробельный символ
    trimmed_text = "".join(trimmed_lines).strip() + "\n"

    # Если после обработки текст пуст — удаляем файл
    if not trimmed_text.strip():
        output_path.unlink(missing_ok=True)
        return False

    # Записываем обработанный текст обратно
    output_path.write_text(trimmed_text, encoding="utf-8")
    return True


def convert_folder(folder: Path, xpdfrc_path: Path, max_workers: int = 1) -> None:
    """
    Находит все PDF-файлы в указанной папке и конвертирует каждый.

    Для каждого PDF создаётся .md файл с тем же именем в той же папке.
    Выводится прогресс и итоговая статистика.

    Если max_workers > 1, конвертация запускается параллельно
    (через ThreadPoolExecutor). Каждый поток запускает свой процесс
    pdftotext.exe, поэтому несколько ядер CPU используются эффективно.
    """
    # Ищем все файлы с расширением .pdf (регистр не важен на Windows)
    pdf_files = sorted(folder.glob("*.pdf"))

    # Если PDF-файлов нет — сообщаем и выходим
    if not pdf_files:
        print(f"PDF-файлы не найдены в папке: {folder}")
        return

    total = len(pdf_files)
    ok = 0      # счётчик успешных конвертаций
    empty = 0   # счётчик пустых результатов (файл удалён)
    fail = 0    # счётчик ошибок

    # Блокировка для безопасного вывода и обновления счётчиков из разных потоков
    print_lock = threading.Lock()

    print(f"Найдено {total} PDF-файл(ов) в: {folder}")
    print(f"Результаты сохраняются в: {folder}")
    if max_workers > 1:
        print(f"Параллельных потоков: {max_workers}")
    else:
        print(f"Режим: последовательный (1 поток)")
    print("-" * 60)

    def process_one(pdf_path: Path, _idx: int) -> dict:
        """Обрабатывает один PDF-файл и возвращает результат."""
        nonlocal ok, empty, fail

        output_path = pdf_path.with_suffix(".md")

        with print_lock:
            print(f"[{_idx}/{total}] Конвертация: {pdf_path.name}")

        success = convert_pdf(pdf_path, output_path, xpdfrc_path)

        if success:
            # Постобработка: обрезаем пробелы в строках,
            # удаляем файл если он пустой
            has_content = trim_output_file(output_path)
            with print_lock:
                if has_content:
                    ok += 1
                    size_kb = output_path.stat().st_size / 1024
                    print(f"[{_idx}/{total}] -> ГОТОВО: {output_path.name} ({size_kb:.1f} КБ)")
                else:
                    empty += 1
                    print(f"[{_idx}/{total}] -> ПУСТОЙ (файл удалён)")
        else:
            with print_lock:
                fail += 1
                print(f"[{_idx}/{total}] -> ОШИБКА")

        return {}

    if max_workers <= 1:
        # Последовательная обработка (как было раньше)
        for idx, pdf_path in enumerate(pdf_files, start=1):
            process_one(pdf_path, idx)
            with print_lock:
                print()
    else:
        # Параллельная обработка через пул потоков
        with concurrent.futures.ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = [
                executor.submit(process_one, pdf_path, idx)
                for idx, pdf_path in enumerate(pdf_files, start=1)
            ]
            # Ждём завершения всех задач
            concurrent.futures.wait(futures)
        print()

    print("-" * 60)
    parts = []
    if ok:
        parts.append(f"{ok} успешно")
    if empty:
        parts.append(f"{empty} пустых")
    if fail:
        parts.append(f"{fail} с ошибками")
    print(f"Завершено: {', '.join(parts)} из {total}")


def main() -> None:
    """
    Точка входа в скрипт.

    Разбирает аргументы командной строки:
      - python pdftotext.py <папка>       - конвертировать PDF в указанной папке
      - python pdftotext.py               - конвертировать PDF в текущей папке
      - python pdftotext.py -j 4          - конвертировать в 4 параллельных потока
      - python pdftotext.py <папка> -j 8  - указать папку и число потоков

    Затем создаёт временный конфиг xpdfrc, запускает конвертацию
    и удаляет временный файл после завершения (даже при ошибках).
    """
    parser = argparse.ArgumentParser(
        description="Конвертер PDF в Markdown с помощью Xpdf pdftotext"
    )
    parser.add_argument(
        "folder",
        nargs="?",          # аргумент необязательный
        default=".",        # по умолчанию — текущая папка
        help="Путь к папке с PDF-файлами (по умолчанию: текущая папка)",
    )
    parser.add_argument(
        "-j", "--jobs",
        type=int,
        default=4,
        help="Количество параллельных потоков (по умолчанию: 4, макс: 10)",
    )
    args = parser.parse_args()

    max_workers = args.jobs
    if max_workers < 1:
        print("ОШИБКА: число потоков (-j) должно быть >= 1", file=sys.stderr)
        sys.exit(1)
    if max_workers > 10:
        print("ОШИБКА: число потоков (-j) не может превышать 10", file=sys.stderr)
        sys.exit(1)

    # Преобразуем в абсолютный путь и проверяем
    folder = Path(args.folder).resolve()

    if not folder.is_dir():
        print(f"ОШИБКА: Папка не найдена: {folder}", file=sys.stderr)
        sys.exit(1)

    print(f"Конвертер PDF -> Markdown")
    print(f"Используется pdftotext: {PDFTOTEXT_EXE}")
    print(f"Папка с PDF:           {folder}")
    print()

    # Проверяем, что все инструменты на месте
    check_prerequisites()

    # Создаём временный файл для конфигурации xpdfrc.
    # Важно: delete=False, потому что pdftotext должен успеть прочитать файл,
    # а NamedTemporaryFile по умолчанию удаляет файл при закрытии.
    with tempfile.NamedTemporaryFile(
        mode="w",
        suffix=".xpdfrc",
        delete=False,
        encoding="utf-8",
    ) as tmp:
        xpdfrc_path = Path(tmp.name)

    try:
        # Записываем конфиг с кириллической поддержкой
        create_xpdfrc(xpdfrc_path)
        # Запускаем конвертацию всех PDF (с указанным числом потоков)
        convert_folder(folder, xpdfrc_path, max_workers=max_workers)
    finally:
        # В любом случае (даже при ошибке) удаляем временный конфиг
        if xpdfrc_path.exists():
            xpdfrc_path.unlink()


if __name__ == "__main__":
    main()
