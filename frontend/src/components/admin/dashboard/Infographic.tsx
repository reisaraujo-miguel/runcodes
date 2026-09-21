import React from "react";

import { Card } from "@/components/ui/card";

interface InfographicProps {
  icon?: React.ReactNode;
  label: string;
  value: number;
}

export function Infographic({ icon, label, value }: InfographicProps) {
  return (
    <Card className="aspect-5/3 max-w-50 min-w-30 p-2 content-contain gap-2 place-content-center">
      <div className="flex justify-end">
        {/* A stat label, not a page heading: an <h1> per card would put several
            top-level headings on the dashboard. */}
        <p className="text-[0.65rem]"> {label} </p>
      </div>
      <div className="grid grid-cols-3 pb-4">
        <div className="grid place-content-center">{icon}</div>
        <div className="grid col-span-2 text-5xl justify-self-center">
          <p> {value} </p>
        </div>
      </div>
    </Card>
  );
}
