import Anthropic from '@anthropic-ai/sdk';
import type { Persona } from './personas';
import type { Rubric } from './rubrics';

export interface DimensionScore {
  dimension: string;
  score: number;
  reasoning: string;
}

export interface EvalResult {
  persona: string;
  rubric: string;
  scores: DimensionScore[];
  averageScore: number;
  commentary: string;
}

const MODEL = process.env.EVAL_MODEL || 'claude-sonnet-4-20250514';

export async function evaluatePost(
  postContent: string,
  postTitle: string,
  persona: Persona,
  rubric: Rubric,
): Promise<EvalResult> {
  const client = new Anthropic();

  const dimensionList = rubric.dimensions
    .map(d => `- "${d.name}": ${d.description}`)
    .join('\n');

  const prompt = `${persona.prompt}

You are evaluating a blog post titled "${postTitle}".

Rate the post on the "${rubric.name}" rubric using these dimensions (each scored 1-5, where 1=poor and 5=excellent):
${dimensionList}

Respond with ONLY valid JSON in this exact format:
{
  "scores": [
    { "dimension": "<name>", "score": <1-5>, "reasoning": "<1-2 sentence explanation>" }
  ],
  "commentary": "<2-3 sentence overall assessment for this rubric>"
}

Here is the blog post:

---
${postContent}
---`;

  const response = await client.messages.create({
    model: MODEL,
    max_tokens: 1024,
    messages: [{ role: 'user', content: prompt }],
  });

  const text = response.content
    .filter(block => block.type === 'text')
    .map(block => block.text)
    .join('');

  const jsonMatch = text.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    throw new Error(`Failed to parse JSON from response:\n${text}`);
  }

  const parsed = JSON.parse(jsonMatch[0]) as {
    scores: DimensionScore[];
    commentary: string;
  };

  const averageScore =
    parsed.scores.reduce((sum, s) => sum + s.score, 0) / parsed.scores.length;

  return {
    persona: persona.slug,
    rubric: rubric.slug,
    scores: parsed.scores,
    averageScore,
    commentary: parsed.commentary,
  };
}
