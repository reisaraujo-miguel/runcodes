import { Input } from "@/components/ui/input";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { LanguageLimits } from "@/lib/api";
import { cn } from "@/lib/utils";

import {
  formatLimitMiB,
  formatLimitSeconds,
  sourceLabel,
} from "./language-limits-format";

/** One language's four inputs, as text ("" = inherit). */
export interface LanguageLimitRow {
  cpu: string;
  mem: string;
  stack: string;
  fileSize: string;
}

/**
 * The per-language limit editor shared by the exercise defaults and a test
 * case's overrides.
 *
 * Each cell shows the value that applies when the field is left empty, so a
 * professor never has to guess what "default" means: the placeholder is the
 * inherited number and the last column names the layer it comes from.
 */
export function LanguageLimitsTable({
  languages,
  values,
  onChange,
  idPrefix,
}: {
  languages: readonly LanguageLimits[];
  values: Record<number, LanguageLimitRow>;
  onChange: (
    languageId: number,
    field: keyof LanguageLimitRow,
    value: string,
  ) => void;
  idPrefix: string;
}) {
  if (languages.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        Nenhuma linguagem permitida neste exercício. Escolha os tipos de arquivo
        aceitos antes de definir limites por linguagem.
      </p>
    );
  }

  return (
    <Table containerClassName="max-h-none" aria-label="Limites por linguagem">
      <TableHeader>
        <TableRow>
          <TableHead>Linguagem</TableHead>
          <TableHead>CPU (s)</TableHead>
          <TableHead>Memória (MiB)</TableHead>
          <TableHead>Pilha (MiB)</TableHead>
          <TableHead>Arquivo (MiB)</TableHead>
          <TableHead>Padrão</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {languages.map((language) => {
          const row = values[language.allowed_file_type_id];
          const inherited = [
            language.sources.cpu_time_limit_seconds,
            language.sources.mem_usage_limit_bytes,
            language.sources.stack_limit_bytes,
            language.sources.file_size_limit_bytes,
          ];
          const layer = inherited.includes("exercise")
            ? "exercício"
            : "plataforma";
          return (
            <TableRow key={language.allowed_file_type_id}>
              <TableCell>
                <p className="font-medium">{language.name}</p>
                <p className="text-xs text-muted-foreground">
                  .{language.extension}
                </p>
              </TableCell>
              <LimitCell
                id={`${idPrefix}-${String(language.allowed_file_type_id)}-cpu`}
                value={row?.cpu ?? ""}
                placeholder={formatLimitSeconds(
                  language.limits.cpu_time_limit_seconds,
                )}
                title={`Padrão de ${sourceLabel(language.sources.cpu_time_limit_seconds)}`}
                onChange={(value) => {
                  onChange(language.allowed_file_type_id, "cpu", value);
                }}
              />
              <LimitCell
                id={`${idPrefix}-${String(language.allowed_file_type_id)}-mem`}
                value={row?.mem ?? ""}
                placeholder={formatLimitMiB(
                  language.limits.mem_usage_limit_bytes,
                )}
                title={`Padrão de ${sourceLabel(language.sources.mem_usage_limit_bytes)}`}
                onChange={(value) => {
                  onChange(language.allowed_file_type_id, "mem", value);
                }}
              />
              <LimitCell
                id={`${idPrefix}-${String(language.allowed_file_type_id)}-stack`}
                value={row?.stack ?? ""}
                placeholder={formatLimitMiB(language.limits.stack_limit_bytes)}
                title={`Padrão de ${sourceLabel(language.sources.stack_limit_bytes)}`}
                onChange={(value) => {
                  onChange(language.allowed_file_type_id, "stack", value);
                }}
              />
              <LimitCell
                id={`${idPrefix}-${String(language.allowed_file_type_id)}-file`}
                value={row?.fileSize ?? ""}
                placeholder={formatLimitMiB(
                  language.limits.file_size_limit_bytes,
                )}
                title={`Padrão de ${sourceLabel(language.sources.file_size_limit_bytes)}`}
                onChange={(value) => {
                  onChange(language.allowed_file_type_id, "fileSize", value);
                }}
              />
              <TableCell className="text-xs whitespace-nowrap text-muted-foreground">
                {layer}
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

function LimitCell({
  id,
  value,
  placeholder,
  title,
  onChange,
}: {
  id: string;
  value: string;
  placeholder: string;
  title: string;
  onChange: (value: string) => void;
}) {
  return (
    <TableCell>
      <Input
        id={id}
        type="number"
        min="0"
        step="0.1"
        value={value}
        placeholder={placeholder}
        title={title}
        aria-label={title}
        className={cn("h-8 min-w-24")}
        onChange={(event) => {
          onChange(event.target.value);
        }}
      />
    </TableCell>
  );
}
