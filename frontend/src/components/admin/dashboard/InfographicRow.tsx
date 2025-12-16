import {
  Cpu,
  Gauge,
  HardDrive,
  LayoutList,
  Terminal,
  Users,
} from "lucide-react";

import { Infographic } from "./Infographic";

export function InfographicRow() {
  const size = "3em";
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 m-4 justify-evenly">
      <Infographic
        icon={<LayoutList size={size} />}
        label="Entregas em Fila"
        value={0}
      ></Infographic>
      <Infographic
        icon={<Terminal size={size} />}
        label="Entregas em Execução"
        value={0}
      ></Infographic>
      <Infographic
        icon={<Users size={size} />}
        label="Usuários Online"
        value={0}
      ></Infographic>
      <Infographic
        icon={<Cpu size={size} />}
        label="Uso de CPU: Server"
        value={0}
      ></Infographic>
      <Infographic
        icon={<HardDrive size={size} />}
        label="Uso de CPU: Base de Dados"
        value={0}
      ></Infographic>
      <Infographic
        icon={<Gauge size={size} />}
        label="Uso de CPU: Compiler"
        value={0}
      ></Infographic>
    </div>
  );
}
