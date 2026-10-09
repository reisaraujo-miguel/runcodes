import type { ReactNode } from "react";

import {
  FILE_INPUT,
  TEXT_INPUT,
  type TestCaseDraft,
} from "@/components/professor/test-case-draft";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

const TYPE_OPTIONS = [
  { value: TEXT_INPUT, label: "Texto" },
  { value: FILE_INPUT, label: "Arquivo" },
];

/**
 * The form fields shared by creating and editing a test case. The edit form
 * hides the type selectors (a case's types are fixed once created) and passes
 * hints explaining how stored files behave.
 */
export function TestCaseFields({
  idPrefix,
  draft,
  onChange,
  typeSelectors = true,
  inputFileHint,
  outputFileHint,
  extraFilesHint,
}: {
  idPrefix: string;
  draft: TestCaseDraft;
  onChange: (patch: Partial<TestCaseDraft>) => void;
  typeSelectors?: boolean;
  inputFileHint?: ReactNode;
  outputFileHint?: ReactNode;
  extraFilesHint?: ReactNode;
}) {
  return (
    <div className="space-y-4">
      {typeSelectors ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-input-type`}>Tipo de entrada</Label>
            <Select
              id={`${idPrefix}-input-type`}
              value={draft.inputType}
              onChange={(event) => {
                onChange({ inputType: event.target.value });
              }}
            >
              {TYPE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-output-type`}>
              Tipo de saída esperada
            </Label>
            <Select
              id={`${idPrefix}-output-type`}
              value={draft.outputType}
              onChange={(event) => {
                onChange({ outputType: event.target.value });
              }}
            >
              {TYPE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </Select>
          </div>
        </div>
      ) : null}

      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-input`}>Entrada</Label>
        {draft.inputType === FILE_INPUT ? (
          <>
            <Input
              id={`${idPrefix}-input`}
              type="file"
              onChange={(event) => {
                onChange({ inputFile: event.target.files?.item(0) ?? null });
              }}
            />
            {inputFileHint}
          </>
        ) : (
          <Textarea
            id={`${idPrefix}-input`}
            value={draft.inputText}
            onChange={(event) => {
              onChange({ inputText: event.target.value });
            }}
            placeholder="Entrada padrão do caso de teste"
          />
        )}
      </div>

      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-output`}>Saída esperada</Label>
        {draft.outputType === FILE_INPUT ? (
          <>
            <Input
              id={`${idPrefix}-output`}
              type="file"
              onChange={(event) => {
                onChange({ outputFile: event.target.files?.item(0) ?? null });
              }}
            />
            {outputFileHint}
          </>
        ) : (
          <Textarea
            id={`${idPrefix}-output`}
            value={draft.outputText}
            onChange={(event) => {
              onChange({ outputText: event.target.value });
            }}
            placeholder="Saída esperada do programa"
          />
        )}
      </div>

      <div className="space-y-2">
        <Label htmlFor={`${idPrefix}-files`}>
          Arquivos adicionais do caso (opcional)
        </Label>
        <Input
          id={`${idPrefix}-files`}
          type="file"
          multiple
          onChange={(event) => {
            onChange({ extraFiles: Array.from(event.target.files ?? []) });
          }}
        />
        {extraFilesHint}
      </div>

      <div className="space-y-2">
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-cpu`}>Tempo limite (s)</Label>
            <Input
              id={`${idPrefix}-cpu`}
              type="number"
              min="0"
              step="0.1"
              value={draft.cpuTime}
              onChange={(event) => {
                onChange({ cpuTime: event.target.value });
              }}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-mem`}>Limite de memória (bytes)</Label>
            <Input
              id={`${idPrefix}-mem`}
              type="number"
              min="0"
              value={draft.memLimit}
              onChange={(event) => {
                onChange({ memLimit: event.target.value });
              }}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-stack`}>Limite de pilha (bytes)</Label>
            <Input
              id={`${idPrefix}-stack`}
              type="number"
              min="0"
              value={draft.stackLimit}
              onChange={(event) => {
                onChange({ stackLimit: event.target.value });
              }}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor={`${idPrefix}-filesize`}>
              Limite de arquivo (bytes)
            </Label>
            <Input
              id={`${idPrefix}-filesize`}
              type="number"
              min="0"
              value={draft.fileSizeLimit}
              onChange={(event) => {
                onChange({ fileSizeLimit: event.target.value });
              }}
            />
          </div>
        </div>
        <p className="text-muted-foreground text-xs">
          0 usa o padrão da plataforma.
        </p>
      </div>

      <div className="flex flex-wrap gap-4">
        <Label htmlFor={`${idPrefix}-show-input`} className="font-normal">
          <Checkbox
            id={`${idPrefix}-show-input`}
            checked={draft.showInput}
            onCheckedChange={(checked) => {
              onChange({ showInput: checked });
            }}
          />
          Mostrar entrada
        </Label>
        <Label htmlFor={`${idPrefix}-show-output`} className="font-normal">
          <Checkbox
            id={`${idPrefix}-show-output`}
            checked={draft.showExpectedOutput}
            onCheckedChange={(checked) => {
              onChange({ showExpectedOutput: checked });
            }}
          />
          Mostrar saída esperada
        </Label>
        <Label htmlFor={`${idPrefix}-show-user`} className="font-normal">
          <Checkbox
            id={`${idPrefix}-show-user`}
            checked={draft.showUserOutput}
            onCheckedChange={(checked) => {
              onChange({ showUserOutput: checked });
            }}
          />
          Mostrar saída do aluno
        </Label>
      </div>
    </div>
  );
}
