import { useTemplates } from "@/hooks/use-templates"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

interface TemplateSelectorProps {
  projectName: string
  value: string
  onChange: (templateName: string) => void
}

export function TemplateSelector({
  projectName,
  value,
  onChange,
}: TemplateSelectorProps) {
  const { data } = useTemplates(projectName)
  const templates = data?.items ?? []

  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger className="w-64">
        <SelectValue placeholder="Select a template" />
      </SelectTrigger>
      <SelectContent>
        {templates.map((t) => (
          <SelectItem key={t.template_name} value={t.template_name}>
            {t.template_name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}
