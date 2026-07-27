export type PublishStepStatus = 'completed' | 'active' | 'pending'

export type PublishWorkflowStep = {
  title: string
  description: string
}

export function publishStepStatus(step: number, currentStep: number, completedSteps: readonly number[]): PublishStepStatus {
  if (step === currentStep) return 'active'
  return completedSteps.includes(step) ? 'completed' : 'pending'
}

export function completePublishStep(completedSteps: readonly number[], step: number) {
  return [...new Set([...completedSteps, step])].sort((left, right) => left - right)
}

export function canVisitPublishStep(step: number, currentStep: number, completedSteps: readonly number[]) {
  return step === currentStep || completedSteps.includes(step)
}

export function firstErrorStep<Field extends string>(
  errors: Partial<Record<Field, string>>,
  fieldSteps: Readonly<Record<Field, number>>,
) {
  for (const field of Object.keys(errors) as Field[]) {
    if (errors[field]) return fieldSteps[field]
  }
  return undefined
}
