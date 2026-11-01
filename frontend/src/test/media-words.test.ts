// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'

/**
 * Returns the compiled catalogue the admin ships for a language.
 * @param locale - The language, such as es-ES.
 * @returns The answers, keyed by the context and the message.
 */
function shipped(locale: string): Record<string, string[]> {
	return JSON.parse(readFileSync(join(repositoryRoot(), 'frontend', 'src', 'languages', `${locale}.json`), 'utf8'))
}

/** The words WordPress's own Spanish and French catalogues give the strings the media library shows. */
const WORDPRESS = [
	{ key: 'Type', es: 'Tipo', fr: 'Type' },
	{ key: 'Thumbnail', es: 'Miniatura', fr: 'Miniature' },
	{ key: 'Title', es: 'Título', fr: 'Titre' },
	{ key: 'No title', es: 'Sin título', fr: 'Aucun titre' },
	{ key: '(no title)', es: '(sin título)', fr: '(pas de titre)' },
	{ key: 'Alt text', es: 'Texto alternativo', fr: 'Texte alternatif' },
	{ key: 'Caption', es: 'Leyenda', fr: 'Légende' },
	{ key: 'Description', es: 'Descripción', fr: 'Description' },
	{ key: 'Date added', es: 'Fecha de inclusión', fr: 'Date d’ajout' },
	{ key: 'Date modified', es: 'Fecha de modificación', fr: 'Date de modification' },
	{ key: 'Author', es: 'Autor', fr: 'Auteur/autrice' },
	{ key: 'File name', es: 'Nombre del archivo', fr: 'Nom de fichier' },
	{ key: 'File size', es: 'Tamaño del archivo', fr: 'Taille du fichier' },
	{ key: 'Dimensions', es: 'Dimensiones', fr: 'Dimensions' },
	{ key: 'File type', es: 'Tipo de archivo', fr: 'Type de fichier' },
	{ key: 'Images', es: 'Imágenes', fr: 'Images' },
	{ key: 'file type group\u0004Audio', es: 'Audio', fr: 'Sons' },
	{ key: 'file type group\u0004Video', es: 'Vídeo', fr: 'Vidéos' },
	{ key: 'Documents', es: 'Documentos', fr: 'Documents' },
	{ key: 'file type group\u0004Archives', es: 'Archivos', fr: 'Archives' },
	{
		key: 'Describe the purpose of the image.',
		es: 'Describe el propósito de la imagen.',
		fr: 'Décrivez la fonction de l’image.',
	},
	{
		key: 'Leave empty if decorative.',
		es: 'Déjalo vacío si es decorativo.',
		fr: 'Laissez vide si l’image est décorative',
	},
	{ key: 'file size\u0004%1$s %2$s', es: '%1$s %2$s', fr: '%1$s %2$s' },
	{ key: 'image dimensions\u0004%1$s × %2$s', es: '%1$s × %2$s', fr: '%1$s par %2$s' },
	{ key: 'unit symbol\u0004B', es: 'B', fr: 'o' },
	{ key: 'unit symbol\u0004KB', es: 'KB', fr: 'Ko' },
	{ key: 'unit symbol\u0004MB', es: 'MB', fr: 'Mo' },
	{ key: 'unit symbol\u0004GB', es: 'GB', fr: 'Go' },
	{ key: 'verb\u0004View', es: 'Ver', fr: 'Voir' },
	{ key: 'Rename…', es: 'Renombrar…', fr: 'Renommer…' },
	{ key: 'Permanently delete…', es: 'Eliminar permanentemente…', fr: 'Supprimer définitivement…' },
	{ key: 'Edit media', es: 'Editar los medios', fr: 'Modifier le média' },
	{ key: 'Rename', es: 'Renombrar', fr: 'Renommer' },
	{ key: 'Name', es: 'Nombre', fr: 'Nom' },
	{ key: 'Save', es: 'Guardar', fr: 'Enregistrer' },
	{ key: 'Cancel', es: 'Cancelar', fr: 'Annuler' },
	{ key: 'Delete permanently', es: 'Borrar permanentemente', fr: 'Supprimer définitivement' },
	{ key: 'Media file updated.', es: 'Archivo de medios actualizado.', fr: 'Fichier média mis à jour.' },
	{
		key: 'Could not save image. %s',
		es: 'No se ha podido guardar la imagen. %s',
		fr: 'Impossible d’enregistrer le média. %s',
	},
	{
		key: 'Could not save media. %s',
		es: 'No se ha podido guardar el archivo multimedia. %s',
		fr: 'Impossible d’enregistrer le média. %s',
	},
	{ key: '"%s" permanently deleted.', es: '«%s» borrado permanentemente.', fr: 'Suppression définitive de « %s ».' },
	{ key: 'Search media', es: 'Buscar medios', fr: 'Rechercher des médias' },
	{ key: 'Add Media File', es: 'Añadir medios', fr: 'Ajouter un fichier média' },
	{
		key: 'Drop files to upload',
		es: 'Arrastra los archivos para subirlos',
		fr: 'Déposez vos fichiers pour les téléverser',
	},
	{ key: 'Media Library', es: 'Biblioteca de medios', fr: 'Médiathèque' },
	{ key: 'Select', es: 'Seleccionar', fr: 'Sélectionner' },
	{ key: 'admin section\u0004Media', es: 'Medios', fr: 'Médias' },
]

/** The rows above, each named by its message and the context it is read in. */
const NAMED = WORDPRESS.map((row) => ({ ...row, named: row.key.split('\u0004').reverse().join(' as ') }))

test.each(NAMED)('ships $named in the Spanish and the French WordPress gives it', ({ key, es, fr }) => {
	expect([shipped('es-ES')[key], shipped('fr-FR')[key]]).toEqual([[es], [fr]])
})
