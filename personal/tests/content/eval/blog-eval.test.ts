import { describe, it, expect } from 'vitest';
import { getPublishedPosts } from '../helpers/parse-posts';
import { personas } from './personas';
import { rubrics } from './rubrics';
import { evaluatePost } from './eval-runner';

const MIN_AVERAGE_SCORE = 2.5;

const posts = getPublishedPosts();
const evalPost = process.env.EVAL_POST;
const evalPersona = process.env.EVAL_PERSONA;

const filteredPosts = evalPost
  ? posts.filter(p => p.slug === evalPost)
  : posts;

const filteredPersonas = evalPersona
  ? personas.filter(p => p.slug === evalPersona)
  : personas;

describe('Blog LLM Evaluation', () => {
  for (const post of filteredPosts) {
    describe(post.slug, () => {
      for (const persona of filteredPersonas) {
        for (const rubric of rubrics) {
          it(
            `${persona.slug} × ${rubric.slug}`,
            async () => {
              const result = await evaluatePost(
                post.content,
                post.frontmatter.title,
                persona,
                rubric,
              );

              console.log(
                `\n[${post.slug}] ${persona.name} × ${rubric.name}: avg=${result.averageScore.toFixed(1)}`,
              );
              for (const s of result.scores) {
                console.log(`  ${s.dimension}: ${s.score}/5 — ${s.reasoning}`);
              }
              console.log(`  Commentary: ${result.commentary}\n`);

              expect(result.averageScore).toBeGreaterThanOrEqual(MIN_AVERAGE_SCORE);
            },
            60_000,
          );
        }
      }
    });
  }
});
